# Technical Notes

- Use hexagonal architecture at all times. Keep business logic isolated from CLI, filesystem, network, storage, and other external adapters.
- Prefer functional testing before calling any feature complete. Unit tests are useful, but they do not prove the tool works the way the design intends.
- Take an agile approach to development. Avoid waterfall: underspecify when useful, prototype early, learn from the result, and refine from working behavior.
- This repo is the provider template (session 001 built it; design/plan in `.journal/001/`). Registry artifact contract, docs-tree rules, and release flow rationale are documented in `.goreleaser.yaml`, `mkdocs.yml`, `moon.yml`, and `DESIGN.md` — read those before changing release or docs machinery.
- Local dev gotchas on this machine: (1) proto's Go shadows mise's in moon's PATH — prefix `PROTO_HOME=$(mktemp -d)` to moon runs; (2) golangci-lint's shared cache leaks stale state across removed worktrees — `golangci-lint cache clean` fixes phantom lint errors; (3) moon filters `runInCI: false` tasks whenever `CI` is set, even for explicit `moon run` — use `env -u CI`.
- Release state: GPG signing key provisioned in Actions secrets (fingerprint 2BE930761ABE8BF747C883124E5F3FC15A6636A7; public key in `.journal/001/gpg-public-key.asc`, registered with no registry). Release Please App credentials set (App ID 3342783; key from 1Password `Development/meigma-release-please`). The template deliberately does not publish releases; release-please will keep recreating its release PR on releasable pushes.
