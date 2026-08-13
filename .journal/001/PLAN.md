# Plan: template-tf-provider

Execution plan for [DESIGN.md](DESIGN.md). Finishing every phase means the
template is ready: a generated repo can implement a real provider and publish
to both registries by following DELETE_ME.md.

## Starting point

This repo is a fresh, un-renamed **template-go instance** (module
`github.com/meigma/template-go`, Cobra/Viper CLI, melange/apko container
path, ghd.toml, mkdocs project under `docs/` with source at `docs/docs/`,
full CI/release/release-dry-run/attest/security-scan/docs-pages workflows,
mise + moon wiring). The plan converts it rather than building greenfield.
Per template-go's own DELETE_ME.md taxonomy, a provider is a **binary-only**
project — the container path goes away — but the binary is a plugin, not a
CLI, and the release artifact contract changes to the registry format.

Layout note discovered during planning: the registries read registry-format
docs from repo-root `docs/` (`docs/index.md`, `docs/resources/…`), but
template-go keeps the mkdocs *project* at `docs/` with its source at
`docs/docs/`. Phase 4 restructures so the shared tree from design D3 lives at
repo-root `docs/` (mkdocs.yml moves to repo root with `docs_dir: docs`; the
docs uv project moves with it).

## Conventions

- Each phase lands as one or more PRs from implementation worktrees off
  `master`, squash-merged; `moon ci` green is the merge bar for every PR.
- Each phase ends in a working state with a functional check (TECH_NOTES:
  functional testing before calling a feature complete), not just unit tests.
- Skills to load during implementation: mise, moonrepo, goreleaser,
  release-please, publishing, go-style, go-testing, github-actions, diataxis.

## Phase 1 — Strip to a provider skeleton

Convert the template-go instance into a minimal building provider repo.

- Rename module to `github.com/meigma/terraform-provider-example`; example
  provider name `example` (binary `terraform-provider-example`).
- Remove the container path: `melange.yaml`, `apko.yaml`, `image-local` mise
  task, `security-scan.yml`, container jobs in `release.yml`/dry-run, and
  the melange/apko pins in `mise.toml` (re-lock).
- Remove ghd distribution (`ghd.toml`, ghd staging scripts) — providers
  install via registries, not ghd.
- Remove the Cobra/Viper CLI (`cmd/template-go`, `internal/cli`,
  `internal/config`); add root `main.go` serving an empty
  `terraform-plugin-framework` provider; drop Cobra/Viper deps, add
  framework deps; `go mod tidy`.
- Add `terraform-registry-manifest.json` (protocol 6.0).
- Keep CI, lint, format, build, test moon tasks green throughout; leave
  release workflows present but not yet correct (Phase 5 owns them).

Exit: `moon run root:check` green; `go build` produces a provider binary
that starts and exits cleanly when invoked by a TF CLI (verified in Phase 2
functional check; here a smoke `--help`/handshake failure message is enough
to prove it's a plugin, not a CLI).

## Phase 2 — Example provider with hexagonal layout

Implement design D2 with a working toy provider.

- `internal/core`: port interfaces + domain logic for a small example
  resource (e.g. an item store with validation/normalization worth testing);
  no framework or HTTP imports; `doc.go`, full godoc.
- `internal/client`: in-memory example adapter implementing the core port
  (stands in for a real API client).
- `internal/provider`: framework provider with one resource, one data
  source, and provider-level config wiring the client in; mirrors what
  scaffolding-framework demonstrates, restructured onto the core seam.
- `.mockery.yml` + generated mocks under `core/mocks` (and `client/mocks`
  if a second port appears); mockery pinned in mise.
- Tests per T1: core unit tests; provider integration tests against core
  mocks.

Exit (functional): with a local CLI config `dev_overrides` pointing at the
built binary, `tofu plan`/`apply`/`destroy` on a sample config in
`examples/` works end to end. `moon ci` green.

## Phase 3 — Acceptance testing (manual-only)

- Add `terraform-plugin-testing` acceptance tests for the example resource
  and data source.
- Pin `opentofu` and `terraform` in mise (re-lock, all four platforms).
- `testacc` moon task: `TF_ACC=1`, `TF_ACC_TERRAFORM_PATH` → mise-pinned
  `tofu` by default; `runInCI: false` — never part of `moon ci`.
- New `.github/workflows/testacc.yml`: `workflow_dispatch` only, with a
  choice input (tofu / terraform / both) driving a matrix over the two
  pinned binaries.

Exit (functional): `moon run root:testacc` passes locally against tofu;
the dispatch workflow run passes for both matrix legs.

## Phase 4 — Docs: one tree for registries and mkdocs

Implement design D3, including the layout restructure.

- Restructure: registry-format tree at repo-root `docs/`; `mkdocs.yml` to
  repo root (`docs_dir: docs`); move the docs uv project (pyproject,
  uv.lock, `.python-version`, docs moon tasks) accordingly; adapt
  `docs-pages.yml`.
- Pin `tfplugindocs` in mise; add `templates/` and wire `examples/`
  (Phase 2's sample configs become the doc examples).
- moon tasks: `docs-gen` (tfplugindocs generate), `docs-check` (regen +
  fail on diff, in `moon ci`), `docs-site` (`mkdocs build --strict`, in CI),
  Pages deploy on default-branch merges.
- Diátaxis skeleton beside the generated pages: `docs/tutorials/`,
  `docs/how-to/`, `docs/explanation/`; mkdocs nav presents generated pages
  as Reference.
- **Resolve the last open question**: verify registry rendering tolerates
  the extra directories (check registry docs/ingestion rules or a known
  provider repo that does this). If not tolerated, fall back to
  `docs/guides/` for Diátaxis content and record the decision in DESIGN.md.

Exit: `moon ci` green with docs gates; Pages site deploys and renders
generated reference + Diátaxis sections; open question closed either way.

## Phase 5 — Release pipeline

Implement designs D5 + D6 on top of template-go's release skeleton.

- `scripts/gpg-provision.sh`: generate dedicated RSA key, push private key
  + passphrase to Actions secrets via `gh secret set`, print armored public
  key + registry registration pointers.
- Rewrite `.goreleaser.yaml` to the registry artifact contract (D1): zip
  naming, binary `terraform-provider-example_v{version}`, manifest asset,
  `_SHA256SUMS`, GPG-signed `_SHA256SUMS.sig`; keep verifiable builds
  (`gomod.proxy`), SBOMs, cosign keyless bundle over `SHA256SUMS`, and
  `actions/attest` with `subject-checksums` (adapt template-go's attest
  flow from container digests to checksum subjects).
- `release-please`: `release-type: go`, `include-v-in-tag`, draft releases,
  App-token trigger chain — adapt template-go's existing workflow; publish
  workflow builds into the draft (`release.mode: keep-existing`) and flips
  draft→public only after all signing/SBOM/attestation succeed.
- Adapt `release-dry-run.yml` as the rehearsal (publishing skill): synthetic
  version, full publish path, never published; runs on release-automation
  changes and on demand. PR CI keeps `goreleaser release --snapshot --clean`
  as the cheap check.
- README/docs get exact consumer verification commands (GPG, cosign
  bundle, `gh attestation verify`).

Exit (functional): rehearsal workflow green end-to-end (artifacts, GPG sig,
cosign bundle, SBOMs, attestation against a draft that is cleaned up);
`terraform providers mirror` or a manual `tofu init` against a
filesystem-mirror layout of the dry-run artifacts proves the zips are
registry-shaped.

## Phase 6 — Template ergonomics and first release

- Rewrite `DELETE_ME.md` for the provider template (D8): rename checklist
  (`example` → real name, module path, goreleaser/release-please/mkdocs/
  workflow/manifest updates), GPG provisioning script step, registry
  onboarding pointers, testacc expectations ("ships manual-only; it is fine
  that it fails until your provider is real").
- Registry onboarding how-to (`docs/how-to/publish.md`): HashiCorp publish
  flow + OpenTofu submission and GPG-key issues (D7).
- Rewrite `README.md` for the template; review CONTRIBUTING/SECURITY;
  update `.github/repository-settings.toml` (required checks changed by
  Phases 1/4/5; drop container-era checks).
- Exercise the release machinery once for real (template-go convention:
  the template proves its own pipeline): merge the release PR, publish
  v0.1.0 of the example provider, verify all three verification paths as a
  consumer would.

Exit: a repo generated from this template can follow DELETE_ME.md from
clone to registry-ready without touching anything the checklist doesn't
name. Template released and pipeline proven.

## Dependencies and sequencing

Strictly sequential 1→2→3→4→5→6 is the default; the real dependencies are:
2 needs 1; 3 needs 2; 4 needs 2 (examples feed tfplugindocs) but not 3;
5 needs 1 only; 6 needs everything. If parallelizing across worktrees, 3/4
and 5 can proceed concurrently after 2.

## Definition of done

- `moon ci` green including docs gates; acceptance workflow green on manual
  dispatch for both tofu and terraform.
- Rehearsal + one real release completed; GPG, cosign, and attestation
  verification commands all succeed against the published release.
- DESIGN.md open questions: zero remaining.
- DELETE_ME.md accurate against the final tree.
