# Design: template-tf-provider

A Meigma GitHub template repository for building Terraform/OpenTofu providers.
It takes HashiCorp's `terraform-provider-scaffolding-framework` as the
functional baseline and replaces its project scaffolding with Meigma
conventions. Where the two conflict, this doc records the decision; where they
don't, we defer to HashiCorp's best practices.

## Goals

- A `terraform-provider-{name}` template that is publishable to both the
  HashiCorp Terraform Registry and the OpenTofu Registry from day one.
- Hexagonal structure where applicable: provider business logic testable
  without Terraform or the upstream API.
- Meigma tooling: mise (toolchain), moon (tasks/CI), release-please
  (versioning), GoReleaser (artifacts), hardened signing/attestation flow,
  mkdocs + GitHub Pages (docs).

## Non-goals

- Supporting the legacy SDKv2. Plugin Framework only.
- A registry mirror or private-registry story; templates target public
  registry publication (private use still works via filesystem/network
  mirrors with no template changes).
- Melange/apko container images. A provider ships as a Go binary in
  registry-format zips; there is no runtime image.

## Decisions

### D1 — Base: Plugin Framework scaffold, kept where it's authoritative

Start from `hashicorp/terraform-provider-scaffolding-framework`:
`terraform-plugin-framework` for the provider, `terraform-plugin-testing` for
acceptance tests, `tfplugindocs` for reference docs, and its `.goreleaser.yml`
as the source of truth for the **registry artifact contract**: per-platform
zips named `terraform-provider-{name}_{version}_{os}_{arch}.zip`, binary
`terraform-provider-{name}_v{version}`, `_SHA256SUMS`, `_SHA256SUMS.sig`
(GPG, detached, binary), and `terraform-registry-manifest.json` uploaded as
`terraform-provider-{name}_{version}_manifest.json`. We do not deviate from
this contract; both registries depend on it.

Dropped from the scaffold: its Makefile, its GitHub Actions workflows, its
docs layout as-is, and its tag-push release trigger. Replaced per D3–D6.

### D2 — Layout: hexagonal, honestly scoped

A provider is mostly two adapters with mapping logic between them. The
template makes that explicit instead of pretending there's a large domain:

```
internal/
  provider/      # adapter: plugin-framework glue (schemas, CRUD wiring)
  core/          # ports + domain logic; no framework or HTTP imports
    mocks/       # mockery-generated (T2/T3)
  client/        # adapter: upstream API client implementing core ports
    mocks/
```

- `core` defines the port interfaces and any real domain rules (validation,
  diff/normalization logic, retry policy). It must compile and test with no
  side effects (A1).
- `provider` translates framework types to core types. Framework resource
  tests that need logic use core mocks, not HTTP fixtures.
- `client` is one adapter for one upstream service (A2). The template ships a
  toy in-memory/example implementation the way the scaffold ships its example
  resource.
- Testing ladder (T1): unit tests in `core`; integration tests of `provider`
  against core mocks; acceptance tests (`TF_ACC=1`) as the end-to-end layer.
- Each package gets `doc.go` (D4) and full godoc (D1).

Scope honesty: for thin CRUD resources the "domain" is mapping + validation.
The template demonstrates the seam without inventing abstraction layers (R1);
trivially thin resources may collapse provider→client and skip core, and the
template README says so.

### D3 — Docs: one `docs/` tree serves the registries and mkdocs

Both registries require registry-format docs at `docs/` (`index.md`,
`resources/`, `data-sources/`, `functions/`, `guides/`). MkDocs also wants a
source tree. We use **one tree**:

- `tfplugindocs` generates `docs/index.md`, `docs/resources/…`,
  `docs/data-sources/…`, `docs/functions/…` from schema + `examples/` +
  `templates/`. These files are generated artifacts: committed (registries
  read the repo), never hand-edited, and CI fails if regeneration produces a
  diff.
- Diátaxis content (D5) lives beside it: `docs/tutorials/`, `docs/how-to/`,
  `docs/explanation/`. **Verified safe** (Phase 4 research): both registries
  use allowlist ingestion and ignore unknown `docs/` subdirectories —
  confirmed in tfplugindocs v0.25.0 source (non-matching dirs are "valid
  non-documentation directories"), by running the validator against a
  Diátaxis fixture (exit 0), and by ~12 published providers including
  HashiCorp's own terraform-provider-tfe (`docs/stylesheets/extra.css`).
  Three constraints hold: never nest inside a recognized dir
  (`docs/resources/<subdir>/` is a hard validation error), Diátaxis content
  is repo/mkdocs-only (registries won't render it; registry-visible prose
  belongs in `docs/guides/` with `page_title`), and never add
  `website/docs/` (triggers HashiCorp's mixed-layout error and makes
  OpenTofu ignore `docs/` entirely). `tfplugindocs validate` passes on this
  layout and acts as the CI guardrail.
- `mkdocs.yml` at repo root, mkdocs + material pinned via uv/python from
  mise. Site deploys to GitHub Pages from a moon task in CI on merges to the
  default branch.
- Registry frontmatter (`page_title`, `subcategory`, `description`) is valid
  mkdocs frontmatter; no transformation step.

### D4 — Tooling: mise owns tools, moon owns tasks

Same split as template-go: `mise.toml` + `mise.lock` (locked, verifying
backends) pin **go, golangci-lint, moon, goreleaser, cosign, tfplugindocs,
mockery, uv/python (mkdocs), opentofu, terraform**. moon tasks run bare
binaries from PATH (`toolchains.default: system`); no tool is installed any
other way.

moon tasks (root project):

- `build`, `lint`, `format`, `test` — as in template-go.
- `docs-gen` — run `tfplugindocs generate`; `docs-check` — regen + fail on
  git diff (CI gate).
- `docs-site` — `mkdocs build --strict`; a deploy task publishes to Pages.
- `testacc` — acceptance tests with `TF_ACC=1`. Never runs automatically:
  excluded from `moon ci` (`runInCI: false`) and exposed only through a
  dedicated `workflow_dispatch`-triggered GitHub Actions workflow. Real
  providers need credentials, create real infrastructure, and cost money;
  manual-only means the template can ship the workflow wired up even while
  the generated project's provider is unimplemented — it simply isn't run
  until someone triggers it.
- CI is `moon ci` behind `jdx/mise-action` (locked, `cache: true` in CI,
  `cache: false` in release workflows), matching template-go's `ci.yml`.

Acceptance tests run against **OpenTofu by default** (via
`TF_ACC_TERRAFORM_PATH` pointing at the mise-pinned `tofu`); the manual
workflow offers a matrix leg for the mise-pinned `terraform` binary so both
ecosystems can be verified from the same trigger.

### D5 — Release flow: release-please orchestrates, tag-triggered publish

Versioning and publication are separate workflows (publishing stance #1):

1. `release-please` (manifest config, `release-type: go`,
   `include-v-in-tag`) runs on pushes to the default branch, maintains the
   release PR and CHANGELOG. Squash-merge policy makes PR titles the
   changelog entries.
2. Merging the release PR makes release-please create the `v*` tag and a
   **draft** GitHub release. It uses a GitHub App token so the tag push
   triggers the publish workflow.
3. `publish` workflow triggers on the tag. mise-action (no cache) installs
   the locked toolchain; GoReleaser builds into the **existing draft
   release** (`release.mode: keep-existing`) with verifiable builds
   (`gomod.proxy: true`).
4. Signing, in order, all subject to D6: GPG-sign `SHA256SUMS` (registry
   contract), cosign keyless bundle-sign `SHA256SUMS`, generate SBOMs for
   shipped archives, then `actions/attest` (pinned) with
   `subject-checksums: SHA256SUMS` for SLSA-oriented provenance in GitHub's
   attestation service.
5. Only after every step succeeds is the release flipped from draft to
   public. Registries ingest only published releases, so a half-signed
   release is never visible to them.
6. A rehearsal workflow (synthetic version, draft-only, never published)
   exercises the same publish path on demand and whenever release automation
   changes. `goreleaser release --snapshot --clean` additionally runs in PR
   CI as a cheap config check.

### D6 — Signing: GPG because the registries require it, keyless on top

The registry protocol verifies GPG only; that key is the one place we accept
long-lived key material. Constraints:

- One dedicated RSA signing key per provider repo (not ECC — unsupported;
  not a personal key). Private key + passphrase live in GitHub environment
  secrets gated to the publish workflow's environment; public key is
  registered in the HashiCorp registry org and submitted to OpenTofu's
  registry.
- A helper script (`scripts/gpg-provision.sh` or similar) generates the key
  and pushes the private key + passphrase to GitHub Actions secrets via
  `gh secret set`, then prints the ASCII-armored public key with pointers to
  the two registry registration steps. Provisioning is one command instead
  of a manual GPG ceremony.
- The GPG key signs exactly one thing: `SHA256SUMS`. Everything else —
  cosign bundles, SBOMs, provenance — uses keyless/OIDC with short-lived
  credentials, so consumers who can do better than GPG get digest-based
  verification (`cosign verify-blob … --bundle`, `gh attestation verify`).
- README documents exact verification commands for both paths.
- Standard hardening throughout: `permissions: {}` at workflow level,
  job-scoped grants, all third-party actions pinned to full SHAs,
  `persist-credentials: false`.

### D7 — Registry onboarding is documentation, not automation

Publishing to each registry has one-time manual steps (HashiCorp publish
flow; OpenTofu submission issue + GPG key issue). The template ships a
Diátaxis how-to (`docs/how-to/publish.md` or README section) walking through
both, rather than trying to automate registry onboarding. Key generation and
secret upload are the exception, handled by the D6 helper script.

### D8 — Bootstrap follows template-go's DELETE_ME.md pattern

The template ships a `DELETE_ME.md` aimed at agents and first-time owners,
mirroring template-go's: what the template provides, how the pieces fit, and
a first-setup checklist — rename the module and provider
(`terraform-provider-scaffolding` → `terraform-provider-{name}`), a
placeholder `rg` sweep, files to update (`.goreleaser.yml`,
`release-please-config.json`, `mkdocs.yml`, workflows,
`terraform-registry-manifest.json`), the GPG provisioning script, registry
onboarding pointers, the full local check, and finally deleting the file
itself. No bootstrap automation beyond that checklist plus the D6 script.

## Repository layout (target)

```
terraform-provider-example/
├── .github/workflows/        # ci, release-please, publish, rehearsal, pages
├── .moon/                    # workspace + toolchains (system)
├── docs/                     # registry docs (generated) + Diátaxis content
├── examples/                 # tf configs feeding tfplugindocs + registry
├── internal/{provider,core,client}/
├── scripts/                  # gpg-provision.sh
├── templates/                # tfplugindocs templates
├── DELETE_ME.md              # agent-facing first-setup checklist (D8)
├── main.go                   # provider server entrypoint
├── terraform-registry-manifest.json
├── mise.toml / mise.lock
├── moon.yml
├── mkdocs.yml
├── .goreleaser.yml
├── release-please-config.json / .release-please-manifest.json
└── go.mod
```

## Open questions

None remaining. The last one (registry rendering of unknown `docs/`
subdirectories) was resolved during Phase 4 — see D3 for the finding and
constraints.

## References

- HashiCorp publishing requirements: developer.hashicorp.com/terraform/registry/providers/publishing
- Scaffold: github.com/hashicorp/terraform-provider-scaffolding-framework
- OpenTofu submission: search.opentofu.org/docs/providers/adding; github.com/opentofu/registry (PROCEDURES.md)
- Meigma precedent: template-go (mise/moon/CI wiring), publishing/release-please/goreleaser skills
