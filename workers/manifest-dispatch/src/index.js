const GO_LIST = "https://go.dev/dl/?mode=json&include=all";
const WORKFLOWS = "https://api.github.com/repos/purpleclay/go-overlay/actions/workflows";
const MAX_BYTES = 1024 * 1024;

// Must match the tool matrix in .github/workflows/tools-update.yml.
export const TOOLS = {
  delve: "github.com/go-delve/delve",
  gofumpt: "mvdan.cc/gofumpt",
  "golangci-lint": "github.com/golangci/golangci-lint/v2",
  gopls: "golang.org/x/tools/gopls",
  govulncheck: "golang.org/x/vuln",
  staticcheck: "honnef.co/go/tools",
};

const minor = (v) => Number(v.match(/^go1\.(\d+)/)[1]);
const isStable = (v) => /^go1\.\d+(\.\d+)?$/.test(v);

// Versions from the newest down to the latest patch of the previous stable minor.
export async function trackedVersions(fetchFn = fetch) {
  const res = await fetchFn(GO_LIST);
  if (!res.ok) throw new Error(`go.dev returned ${res.status}`);
  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  const re = /"version":\s*"(go[^"]+)"/g;
  const versions = [];
  let text = "";
  let pos = 0; // a failed exec resets lastIndex, so remember where the last match ended
  try {
    while (text.length < MAX_BYTES) {
      const { done, value } = await reader.read();
      if (done) break;
      text += decoder.decode(value, { stream: true });
      re.lastIndex = pos;
      let m;
      while ((m = re.exec(text))) {
        pos = re.lastIndex;
        const v = m[1];
        if (versions.at(-1) !== v) versions.push(v);
        const latest = versions.find(isStable);
        if (latest && isStable(v) && minor(v) === minor(latest) - 1) return versions;
      }
    }
  } finally {
    await reader.cancel();
  }
  throw new Error(`no previous stable minor in the first ${text.length} bytes`);
}

// The same list goscrape reads. The proxy does not promise an order, so sort it.
async function moduleVersions(module, fetchFn) {
  const res = await fetchFn(`https://proxy.golang.org/${module}/@v/list`);
  if (!res.ok) throw new Error(`proxy returned ${res.status} for ${module}`);
  return (await res.text()).split("\n").filter(Boolean).sort();
}

async function dispatch(env, workflow, inputs, fetchFn) {
  const res = await fetchFn(`${WORKFLOWS}/${workflow}/dispatches`, {
    method: "POST",
    headers: {
      Accept: "application/vnd.github+json",
      Authorization: `Bearer ${env.GITHUB_TOKEN}`,
      "User-Agent": "go-overlay-manifest-dispatch",
      "X-GitHub-Api-Version": "2022-11-28",
    },
    body: JSON.stringify({ ref: "main", ...(inputs && { inputs }) }),
  });
  if (!res.ok) throw new Error(`${workflow} dispatch returned ${res.status}: ${await res.text()}`);
}

// Each cron checks one group, so each gets its own CPU budget. Must match wrangler.jsonc.
export const CRONS = {
  "*/15 * * * *": "go",
  "7-59/15 * * * *": "tools",
};

const groups = {
  go: [{ key: "go", workflow: "go-update.yml", versions: (f) => trackedVersions(f) }],
  tools: Object.entries(TOOLS).map(([tool, module]) => ({
    key: tool,
    workflow: "tools-update.yml",
    inputs: { tool },
    versions: (f) => moduleVersions(module, f),
  })),
};

// Dispatches a workflow for each source in the group whose versions changed. Returns the keys it dispatched.
export async function check(env, group, fetchFn = fetch) {
  const dispatched = [];
  const errors = [];
  for (const { key, workflow, inputs, versions } of groups[group]) {
    try {
      const current = (await versions(fetchFn)).join(" ");
      if (current === (await env.STATE.get(key))) continue;
      await dispatch(env, workflow, inputs, fetchFn);
      // Only after a successful dispatch, so a failure is retried on the next tick.
      await env.STATE.put(key, current);
      dispatched.push(key);
    } catch (e) {
      errors.push(e);
    }
  }
  if (errors.length) throw new AggregateError(errors, errors.map((e) => e.message).join("; "));
  return dispatched;
}

export default {
  async scheduled(controller, env) {
    const group = CRONS[controller.cron];
    if (!group) throw new Error(`no group for cron ${controller.cron}`);
    const dispatched = await check(env, group);
    if (dispatched.length) console.log(`dispatched: ${dispatched.join(", ")}`);
  },
};
