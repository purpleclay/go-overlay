import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import worker, { check, CRONS, TOOLS, trackedVersions } from "../src/index.js";

const head = JSON.parse(readFileSync(new URL("go-dev-head.json", import.meta.url), "utf8"));
const tracked = "go1.27.4 go1.27.3 go1.27.2 go1.27.1 go1.27.0 go1.27rc3 go1.27rc2 go1.27rc1 go1.26.11";
const toolVersions = Object.fromEntries(Object.keys(TOOLS).map((t) => [t, ["v1.0.0", "v1.1.0"]]));
const allSeen = () => ({ go: tracked, ...Object.fromEntries(Object.keys(TOOLS).map((t) => [t, "v1.0.0 v1.1.0"])) });

// Small chunks so versions are split across reads, as they are over the network.
function streamed(text, size = 100) {
  const bytes = new TextEncoder().encode(text);
  return new ReadableStream({
    start(c) {
      for (let i = 0; i < bytes.length; i += size) c.enqueue(bytes.slice(i, i + size));
      c.close();
    },
  });
}

function fakeFetch({ releases = head, goList, tools = {}, status = {} } = {}) {
  const calls = [];
  const fn = async (url, init) => {
    calls.push({ url, init });
    if (url.startsWith("https://go.dev/")) {
      return goList ?? new Response(streamed(JSON.stringify(releases, null, 1)));
    }
    if (url.startsWith("https://proxy.golang.org/")) {
      const tool = Object.keys(TOOLS).find((t) => url === `https://proxy.golang.org/${TOOLS[t]}/@v/list`);
      const versions = tools[tool] ?? toolVersions[tool];
      return typeof versions === "number" ? new Response("", { status: versions }) : new Response(versions.join("\n") + "\n");
    }
    const body = JSON.parse(init.body);
    return new Response(null, { status: status[body.inputs?.tool ?? "go"] ?? 204 });
  };
  fn.urls = () => calls.map((c) => c.url);
  fn.dispatches = () =>
    calls
      .filter((c) => c.url.startsWith("https://api.github.com/"))
      .map((c) => ({ ...c, workflow: c.url.split("/").at(-2), body: JSON.parse(c.init.body) }));
  return fn;
}

function fakeEnv(seen = allSeen()) {
  const kv = new Map(Object.entries(seen));
  return {
    GITHUB_TOKEN: "test-token",
    STATE: { get: async (k) => kv.get(k) ?? null, put: async (k, v) => void kv.set(k, v) },
    seen: () => Object.fromEntries(kv),
  };
}

const release = (version, stable) => ({ version, stable, files: [{ version }] });

test("tracked versions stop at the latest patch of the previous stable minor", async () => {
  assert.equal((await trackedVersions(fakeFetch())).join(" "), tracked);
});

test("tracked versions include a new minor's release candidates above the latest stable", async () => {
  const releases = [release("go1.28rc1", false), ...head];
  assert.equal((await trackedVersions(fakeFetch({ releases }))).join(" "), `go1.28rc1 ${tracked}`);
});

for (const group of ["go", "tools"]) {
  test(`unchanged ${group} versions do not dispatch or write`, async () => {
    const fetch = fakeFetch();
    const env = fakeEnv();
    env.STATE.put = () => assert.fail("wrote to KV");
    assert.deepEqual(await check(env, group, fetch), []);
    assert.equal(fetch.dispatches().length, 0);
  });
}

test("the go group reads only go.dev and the tools group only the proxy", async () => {
  const goFetch = fakeFetch();
  await check(fakeEnv(), "go", goFetch);
  const toolsFetch = fakeFetch();
  await check(fakeEnv(), "tools", toolsFetch);
  assert.deepEqual(goFetch.urls(), ["https://go.dev/dl/?mode=json&include=all"]);
  assert.ok(toolsFetch.urls().every((u) => u.startsWith("https://proxy.golang.org/")));
  assert.equal(toolsFetch.urls().length, Object.keys(TOOLS).length);
});

test("a reordered proxy list does not dispatch", async () => {
  const fetch = fakeFetch({ tools: { gopls: ["v1.1.0", "v1.0.0"] } });
  assert.deepEqual(await check(fakeEnv(), "tools", fetch), []);
});

test("new Go version dispatches go-update once and stores the list", async () => {
  const fetch = fakeFetch({ releases: [release("go1.27.5", true), ...head] });
  const env = fakeEnv();
  assert.deepEqual(await check(env, "go", fetch), ["go"]);

  const [call, ...rest] = fetch.dispatches();
  assert.equal(rest.length, 0);
  assert.equal(call.url, "https://api.github.com/repos/purpleclay/go-overlay/actions/workflows/go-update.yml/dispatches");
  assert.equal(call.init.method, "POST");
  assert.equal(call.init.headers.Authorization, "Bearer test-token");
  assert.deepEqual(call.body, { ref: "main" });
  assert.equal(env.seen().go, `go1.27.5 ${tracked}`);
});

test("new tool version dispatches tools-update for that tool only", async () => {
  const fetch = fakeFetch({ tools: { gopls: ["v1.0.0", "v1.1.0", "v1.2.0-pre.1"] } });
  const env = fakeEnv();
  assert.deepEqual(await check(env, "tools", fetch), ["gopls"]);

  const [call, ...rest] = fetch.dispatches();
  assert.equal(rest.length, 0);
  assert.equal(call.workflow, "tools-update.yml");
  assert.deepEqual(call.body, { ref: "main", inputs: { tool: "gopls" } });
  assert.deepEqual(env.seen(), { ...allSeen(), gopls: "v1.0.0 v1.1.0 v1.2.0-pre.1" });
});

test("first run with nothing stored dispatches every workflow", async () => {
  const env = fakeEnv({});
  assert.deepEqual(await check(env, "go", fakeFetch()), ["go"]);
  assert.deepEqual(await check(env, "tools", fakeFetch()), Object.keys(TOOLS));
  assert.deepEqual(env.seen(), allSeen());
});

for (const status of [401, 500]) {
  test(`failed go-update dispatch (${status}) throws and keeps the stored list`, async () => {
    const env = fakeEnv({ ...allSeen(), go: "go1.27.3" });
    await assert.rejects(check(env, "go", fakeFetch({ status: { go: status } })), new RegExp(`go-update.yml dispatch returned ${status}`));
    assert.equal(env.seen().go, "go1.27.3");
  });
}

test("failed dispatch for one tool keeps its list and still stores the others", async () => {
  const env = fakeEnv({ ...allSeen(), gopls: "v1.0.0", delve: "v1.0.0" });
  await assert.rejects(check(env, "tools", fakeFetch({ status: { gopls: 500 } })), /tools-update.yml dispatch returned 500/);
  assert.equal(env.seen().gopls, "v1.0.0");
  assert.equal(env.seen().delve, "v1.0.0 v1.1.0");
});

test("go.dev error throws without dispatching", async () => {
  const fetch = fakeFetch({ goList: new Response("busy", { status: 503 }) });
  await assert.rejects(check(fakeEnv({}), "go", fetch), /go.dev returned 503/);
  assert.equal(fetch.dispatches().length, 0);
});

test("proxy error for one tool throws but still checks the others", async () => {
  const fetch = fakeFetch({ tools: { staticcheck: 404 } });
  await assert.rejects(check(fakeEnv({}), "tools", fetch), /proxy returned 404 for honnef.co\/go\/tools/);
  const tools = fetch.dispatches().map((d) => d.body.inputs.tool);
  assert.deepEqual(tools, Object.keys(TOOLS).filter((t) => t !== "staticcheck"));
});

test("a Go list with no previous stable minor in the first 1 MB throws without dispatching go-update", async () => {
  const releases = Array.from({ length: 20000 }, (_, i) => release(`go1.27.${i}`, true));
  const fetch = fakeFetch({ releases });
  await assert.rejects(check(fakeEnv(), "go", fetch), /no previous stable minor/);
  assert.equal(fetch.dispatches().length, 0);
});

test("tools match the tools-update workflow", () => {
  const workflow = readFileSync(new URL("../../../.github/workflows/tools-update.yml", import.meta.url), "utf8");
  const listed = [...workflow.matchAll(/\{ "name": "([^"]+)", "module": "([^"]+)"/g)].map((m) => [m[1], m[2]]);
  assert.deepEqual(Object.fromEntries(listed), TOOLS);
});

test("crons match wrangler.jsonc", () => {
  const config = readFileSync(new URL("../wrangler.jsonc", import.meta.url), "utf8").replace(/^\s*\/\/.*$/gm, "");
  assert.deepEqual(JSON.parse(config).triggers.crons, Object.keys(CRONS));
});

test("an unknown cron throws", async () => {
  await assert.rejects(worker.scheduled({ cron: "0 0 * * *" }, fakeEnv()), /no group for cron 0 0 \* \* \*/);
});

test("each cron checks its own group", async (t) => {
  for (const [cron, host] of [["*/15 * * * *", "go.dev"], ["7-59/15 * * * *", "proxy.golang.org"]]) {
    const fetch = fakeFetch();
    t.mock.method(globalThis, "fetch", fetch);
    await worker.scheduled({ cron }, fakeEnv());
    assert.ok(fetch.urls().length > 0 && fetch.urls().every((u) => new URL(u).host === host), cron);
    t.mock.restoreAll();
  }
});
