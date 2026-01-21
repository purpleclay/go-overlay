# Contributing to go-overlay

Thank you for your interest in contributing to go-overlay! Whether you're fixing a bug, adding a feature, or improving documentation, your help is welcome and appreciated.

Please take a moment to read through this guide before submitting your contribution. It helps maintain a consistent workflow and ensures a smooth review process for everyone involved.

By participating in this project, you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md).

## AI Assistance

AI-assisted contributions are welcome, but please be transparent about their use. If you've used AI tools to help write code, mention it in your pull request description. You should understand and be able to explain any code you submit, as reviewers may ask questions about implementation decisions.

An example disclosure:

> This PR was written primarily by Claude Code.

Or a more detailed disclosure:

> I consulted ChatGPT to understand the codebase but the solution was authored manually by myself.

AI-generated content without human review or understanding is not acceptable. You are accountable for the code you contribute.

## Getting Started

This is a Nix-based project. To get started, you'll need Nix installed on your machine.

### Installing Nix

We recommend using the [Determinate Systems Nix Installer](https://github.com/DeterminateSystems/nix-installer):

```sh
curl --proto '=https' --tlsv1.2 -sSf -L https://install.determinate.systems/nix | sh -s -- install
```

### Development Environment

Once Nix is installed, enter the development shell:

```sh
nix develop
```

This provides everything you need: Go, formatters, linters, and pre-commit hooks.

### Validating Changes

#### Go Changes

Build and test your Go changes:

```sh
go build ./...
go test ./...
```

Run the linter:

```sh
golangci-lint run
```

Format your code:

```sh
gofumpt -w .
```

If you've changed dependencies in `go.mod`, regenerate the vendor manifest:

```sh
govendor
```

#### Nix Changes

Format Nix files:

```sh
alejandra .
```

Run the Nix checks:

```sh
nix flake check
```

Build packages:

```sh
nix build .#govendor
nix build .#goscrape
```

Run the integration tests:

```sh
nix build .#integration-build-go-module
nix build .#integration-indirect-deps
nix build .#integration-local-replace
nix build .#integration-workspace-api
nix build .#integration-workspace-worker
nix build .#integration-workspace-no-gowork
```

## Commits

This project follows the [Conventional Commits](https://www.conventionalcommits.org/) specification. Your commit messages should be structured as:

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

Common types include `feat`, `fix`, `docs`, `refactor`, `test`, and `chore`.

### Developer Certificate of Origin

All commits must be signed off to certify that you have the right to submit the code under the project's license. This is done by adding a `Signed-off-by` line to your commit message:

```
Signed-off-by: Your Name <your.email@example.com>
```

Use `git commit -s` to automatically add this line.

### Single Commit

Please squash your changes into a single commit before submitting. This keeps the history clean and makes it easier to review, revert, or cherry-pick changes if needed.

```sh
git rebase -i HEAD~<number-of-commits>
```

## Pull Requests

Before opening a pull request:

1. Ensure your changes build and pass all tests
2. Format your code with the appropriate tools
3. Squash your changes into a single, well-structured commit
4. Write a clear PR description explaining what the change does and why

Pull requests should be focused and address a single concern. If you're fixing multiple unrelated issues, please submit separate PRs.

Expect feedback during the review process. This is collaborative, not adversarial. Be open to suggestions and willing to iterate on your changes.
