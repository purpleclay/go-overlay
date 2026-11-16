# manifest-dispatch

GitHub often starts scheduled workflows hours late, or skips them. This Cloudflare Worker checks go.dev and the Go module proxy every 15 minutes, and dispatches `go-update` or `tools-update` when a new Go or tool release appears. goscrape still decides which manifests to generate. The Worker only notices that something changed.

Adding a tool to `tools-update.yml` means adding it to `TOOLS` in `src/index.js` too. A test fails if they differ:

```sh
nix run --inputs-from ../.. nixpkgs#nodejs -- --test
```
