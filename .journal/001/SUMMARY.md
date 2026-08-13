---
id: 001
title: Convert template-go baseline into template-tf-provider
date: 2026-08-13
status: complete
repos_touched: [template-tf-provider]
related_sessions: []
---

## Goal

Design and build Meigma's Terraform/OpenTofu provider template by converting
this repo's template-go baseline: HashiCorp's scaffolding-framework functional
core wrapped in Meigma conventions (hexagonal layout, mise/moon, mkdocs +
GitHub Pages, release-please, hardened signing/attestation release flow).

## Outcome

Met, with one deliberate scope change. All six plan phases landed as
squash-merged PRs (#6, #8, #9 + hotfix #10, #11, #14 + hotfix #15, #16), every
exit criterion was verified functionally (dev_overrides lifecycle, both-CLI
acceptance workflow run, docs drift gates, CI dry-run rehearsal with the real
signing key, filesystem-mirror `tofu init`/`terraform init` against real
artifacts). The planned v0.1.0 self-release was intentionally NOT published:
the user decided real releases are for generated projects; release PR #17
(correctly computed as 0.1.0) was closed unmerged. The pipeline is proven by
the green rehearsal instead.

## Key Decisions

- Design/plan live in this folder (DESIGN.md, PLAN.md); all design open
  questions resolved to zero.
- File-backed example store (not in-memory) -> state must survive across CLI
  invocations for plan/apply/destroy to mean anything; keeps acceptance tests
  hermetic.
- Core rule the template teaches: validation rejects, normalization limited to
  set-safe sort+dedupe -> a provider that rewrites configured values fails its
  own apply.
- Acceptance tests manual-only (user decision): `runInCI: false` + dispatch-only
  workflow; workflow must clear `CI` env because moon filters runInCI:false
  tasks even for explicit `moon run`.
- Diátaxis dirs directly under `docs/` -> proven safe (registries allowlist and
  ignore unknown dirs; tfplugindocs generate wipes `docs/guides/`, so the
  fallback would have destroyed prose). Never nest inside recognized dirs;
  never add `website/docs/`.
- GPG stays (registry trust anchor) but signs only SHA256SUMS; cosign
  bundle/SBOMs/attestations layered keylessly; cosign isolated in its own job
  so the build job never holds an OIDC token.
- `release.disable: true` -> Release Please owns the release object; draft
  flips public only after all jobs succeed (replaces template-go's manual
  inspection gate).
- gomod.proxy (verifiable builds) deliberately off -> template module path
  cannot resolve via Go proxy and the pipe is unrehearsable; generated repos
  enable it post-rename (documented in .goreleaser.yaml).
- release-please manifest `0.0.0` is a "never released" sentinel that ignores
  bump flags and cuts 1.0.0 -> `initial-version: 0.1.0` is the correct knob;
  CHANGELOG must be genuinely empty (leftover heading becomes a permanent
  stray).
- Orchestration: guided Opus subagents implemented; orchestrator reviewed every
  diff, re-ran verification independently, and owned git/PRs.

## Changes

All in this repo; headline files per phase:
- Phase 1 (#6): module -> terraform-provider-example; CLI/container/ghd removed;
  root main.go + internal/provider skeleton (framework v1.19.0, protocol 6);
  terraform-registry-manifest.json; CVE pins (grpc 1.83.0, x/net 0.58.0).
- Phase 2 (#8): internal/{core,client,provider} example_item CRUD+import +
  data source; mockery mock; testify tests; examples/ in tfplugindocs layout;
  mise pins mockery, opentofu.
- Phase 3 (#9, #10): acceptance_test.go (plugin-testing v1.16.0; TestMain sets
  TF_ACC_PROVIDER_NAMESPACE=meigma — OpenTofu rejects the library's legacy '-'
  reattach address); testacc task + testacc.yml (CI:'' bypass, -count=1 to
  defeat Go test cache); terraform pin.
- Phase 4 (#11): docs/ single tree (mkdocs.yml at root, docs_dir: docs);
  tfplugindocs 0.25.0 + docs-gen/docs-check/docs-validate gates; Diátaxis
  content incl. walked tutorial; golangci-lint mutex; pymdown CVE fix; uv
  exclude-newer 7-day window.
- Phase 5 (#14, #15): .goreleaser.yaml registry contract (13 zips, inner _v
  binary, manifest asset, SHA256SUMS + binary GPG sig, SBOMs); release.yml
  contract flow + publish-release flip; shared scripts/ (stage/check/smoke);
  scripts/gpg-provision.sh + regression test (fake gh must mimic real CLI:
  `--body -` stores a literal dash — bug caught by the CI rehearsal, fixed);
  attest.yml container path removed.
- Phase 6 (#16): DELETE_ME.md rewritten (verified rename table), README
  rewrite, docs/how-to/publish.md (registry onboarding), CONTRIBUTING/SECURITY
  updates, CHANGELOG/manifest reset + initial-version 0.1.0.

## Open Threads

- Release PR #17 closed unmerged (user decision: no template self-release).
  release-please will recreate it on future releasable pushes; a real release
  happens only in generated projects (or if that decision changes).
- GPG public key (fingerprint 2BE930761ABE8BF747C883124E5F3FC15A6636A7, saved
  as gpg-public-key.asc here) is provisioned in Actions secrets but NOT
  registered with either registry — the template itself is not published.
- `create-github-app-token` warns app-id input is deprecated; switch
  release-please.yml to `client-id` (value exists in the 1Password item).
- Dependabot PRs #1/#2/#3/#13 open; #2 targets the removed /docs uv dir and is
  obsolete.
- release-dry-run.yml triggers only on `master` while ci/docs-pages also list
  `main` — a generated repo defaulting to `main` would silently lose the
  required dry-run check.
- `bump-patch-for-minor-pre-major: true` (inherited): pre-1.0 features bump
  patch only; user may want a conscious yes/no.
- golangci-lint exit-3 format/lint flake seen once post-mutex; mutex narrowed
  but may not have eliminated it.

## Lessons

- Rehearse in the target environment: two bugs passed local verification and
  were caught only by running the real workflow (moon's CI filtering; gh
  secret set `--body -`). Corollary: a fake tool must mimic the real CLI's
  documented behavior, not the caller's assumption — a mock sharing the code's
  misconception is a tautology.
- GitHub log masking replacing every hyphen with *** was the tell that a
  secret's value was literally `-`.
- Verify version tooling against source: release-please's 0.0.0 sentinel and
  changelog-heading demotion both contradicted reasonable doc-level
  assumptions.

## References

- PRs: #6, #8, #9, #10, #11, #14, #15, #16 (merged); #17 (closed unmerged).
- DESIGN.md, PLAN.md, gpg-public-key.asc in this folder.
- Key upstream sources: hashicorp/terraform-provider-scaffolding-framework;
  developer.hashicorp.com/terraform/registry/providers/{publishing,docs};
  opentofu/registry PROCEDURES + issue templates; terraform-plugin-docs
  v0.25.0 validate/generate source.
