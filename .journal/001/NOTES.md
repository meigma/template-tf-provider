---
id: 001
title: First session in template-tf-provider
started: 2026-08-12
---

## 2026-08-12 16:29 — Kickoff
Goal for the session: not yet stated — the user started a new session and has not given a task.
Current state of the world: fresh repo from template (single commit "Initial commit" on master, clean tree). Journal scaffold exists (INDEX.md, SKILLS.md, TECH_NOTES.md) with no prior sessions. Journal worktree `journal/jmgilman` at `.wt/journal-jmgilman` is in sync with origin.
Plan: wait for the user's request, then journal at meaningful checkpoints.

## 2026-08-12 16:35 — Research: provider publishing paths
User asked three questions ahead of writing a design doc. Findings:
- Terraform Registry: public GitHub repo `terraform-provider-{name}`, semver `v*` tags, GoReleaser-style release assets (zips, SHA256SUMS, GPG .sig, manifest.json), GPG key registered in registry account, docs in repo; registry ingests via webhook after one-time publish flow.
- OpenTofu: separate registry (github.com/opentofu/registry). Submission = GitHub issue from a template; automation validates and opens a PR; maintainers merge. Anyone can submit, not just the author. GPG key submission requires public org membership + separate issue. Same repo naming convention.
- Private/local use: dev_overrides, filesystem_mirror (packed/unpacked layouts), network_mirror protocol, implied plugin dirs (~/.terraform.d/plugins), or a private registry implementing the provider registry protocol.
Next: write small design doc in .journal/001/.

## 2026-08-12 16:48 — Design doc drafted
Wrote DESIGN.md in this session folder: scaffolding-framework as base, hexagonal internal/{provider,core,client} split, single docs/ tree shared by registries + mkdocs, mise/moon tooling, release-please -> draft release -> tag-triggered GoReleaser publish with GPG (registry contract) + cosign/SBOM/attest on top, flip draft->public last. Open questions recorded (registry tolerance of extra docs dirs, rename script, GPG provisioning, testacc cadence).

## 2026-08-12 16:58 — Design feedback round 1
User decisions: bootstrap follows template-go's DELETE_ME.md agent-facing checklist (now D8); GPG provisioning gets a helper script that generates the key and pushes secrets via `gh secret set` (folded into D6). Open questions trimmed to two: registry tolerance of extra docs/ dirs, and testacc CI cadence (explained to user, awaiting their call).

## 2026-08-12 17:05 — Acceptance test cadence decided
User decision: acceptance tests never run automatically. testacc is excluded from moon ci and shipped as a workflow_dispatch-only GitHub Actions workflow; shipping "technically broken" until the real provider exists is fine since nothing triggers it. Design doc updated (D4); only remaining open question is registry tolerance of extra docs/ dirs.

## 2026-08-12 17:15 — Plan written
Discovered the repo is a fresh template-go instance (not empty): Cobra CLI, melange/apko, ghd.toml, mkdocs project at docs/ with source at docs/docs/, full workflows. Plan is therefore a conversion. Wrote PLAN.md next to DESIGN.md: 6 phases — (1) strip to provider skeleton (drop container path/ghd/CLI, framework main.go), (2) hexagonal example provider + dev_overrides functional check, (3) manual-only acceptance testing (tofu default, terraform matrix), (4) docs restructure to repo-root docs/ shared by registries + mkdocs (resolves last open question), (5) release pipeline (registry artifact contract + GPG script + cosign/SBOM/attest, rehearsal), (6) DELETE_ME.md rewrite + first real release. Noted docs layout conflict: registries need repo-root docs/index.md; template-go has mkdocs source at docs/docs/ — Phase 4 moves mkdocs.yml to root with docs_dir: docs.

## 2026-08-12 17:22 — Phase 1 execution started
Created implementation worktree feat/provider-skeleton (.wt/feat-provider-skeleton) off fetched master. Spawned a guided Opus subagent to do the conversion: module rename to terraform-provider-example, remove CLI/container path/ghd, add framework provider skeleton + registry manifest, minimal goreleaser/moon/workflow consistency patches, mise re-lock. I stay orchestrator/reviewer: will review diff, commit, and open the PR. Verification bar: moon run root:check green + plugin smoke test.

## 2026-08-12 17:38 — Phase 1 implemented, PR #6 open
Opus subagent completed the conversion; I reviewed the full diff, re-ran root:check (7/7 green) and plugin smoke tests myself. Two review fixes applied: release-please-config (stale melange/apko extra-files + package name — would have broken next release PR) and a moon.yml comment inaccuracy. Committed as feat/provider-skeleton, PR #6 opened (squash target). Notables: agent caught GoReleaser formats:binary clobbering trap in asset staging (fixed with artifacts.json-path staging + 9-asset assertion); pre-existing local proto/mise Go conflict breaks moon runs outside CI (workaround: PROTO_HOME to empty dir; fix separately). attest.yml dead container inputs deferred to Phase 5. Framework v1.19.0, protocol 6.

## 2026-08-12 17:52 — PR #6 all checks green
Kusari flagged two reachable transitive CVEs from terraform-plugin-framework (grpc v1.79.2 critical CVE-2026-33186 + GO-2026-6061; x/net v0.48.0 high CVE-2026-39821). Subagent pinned grpc v1.83.0 and x/net v0.58.0 via MVS (x/sys, x/text, genproto/rpc moved forward as side effects); I reviewed, committed (deps:), pushed. PR #6 now: ci pass, Kusari pass, Pages pass, dry-run skip-as-designed. Awaiting user review/squash-merge before Phase 2.

## 2026-08-12 18:02 — Phase 1 merged; Phase 2 started
User approved: PR #6 squash-merged (master e7841bf); phase-1 worktree and remote branch removed. Created feat/example-provider worktree; spawned Opus subagent for Phase 2 (hexagonal example provider). Notable brief decisions: backing store is FILE-BACKED (JSON, atomic rename) not in-memory, so state survives across CLI invocations and acceptance tests stay hermetic; provider config store_path attr with EXAMPLE_STORE_PATH fallback; opentofu mise pin pulled forward from Phase 3 because Phase 2's dev_overrides functional exit needs it; mockery added via mise with mock for core.Store in core/mocks.

## 2026-08-12 18:26 — Phase 2 review: approved + import addition
Subagent delivered: core (typed Item, validation rejects / normalization limited to set sort+dedupe for plan consistency), client (atomic JSON file store, in-process mutex, crypto/rand IDs), provider (example_item resource + data source, attribute-scoped diags keyed off core.Field). Functional transcript clean: plan/apply/no-drift-replan/update-same-id/invalid-input-diag/destroy-empty-store. Zero lint exclusions. I re-ran build+tests independently and read the three key files in full — approved. One addition requested before commit: ResourceWithImportState + ImportStatePassthroughID, import test, examples import.sh, functional import check (concern #4 from agent — template users expect import). Also noted: mockery 3.7.2 + opentofu 1.12.5 pins carry checksum but no provenance line in mise.lock (neither publishes attestations aqua records) — a notch below other tools, acceptable. Stale-LSP false diagnostics again (indexes removed phase-1 worktree path) — verified against real builds instead.

## 2026-08-12 18:58 — Phase 2 complete, PR #8 open and green
Import addition delivered and verified (ImportStatePassthroughID; test drives ImportState→Read; tofu import worked under dev_overrides with no init needed; idAttribute const extracted to stay under goconst threshold). I re-verified: root:check 7/7, go test clean, import.sh present. Committed feat/example-provider (30 files, +3071), PR #8 opened; all checks green (ci, Kusari, Pages; dry-run skip-by-design). Awaiting user review/merge. Next: Phase 3 (manual-only acceptance testing) — remember opentofu pin already landed in Phase 2, so Phase 3 adds terraform pin + plugin-testing + testacc task + dispatch workflow.

## 2026-08-12 19:05 — Phase 2 merged; Phase 3 started
PR #8 squash-merged (master 661efa0); worktree/branch cleaned. Created feat/acceptance-tests worktree; spawned Opus subagent for Phase 3: terraform-plugin-testing acceptance suite (create/read, update-keeps-id, ImportStateVerify, data source, CheckDestroy), terraform mise pin, testacc moon task (runInCI false, TF_ACC_TERRAFORM_PATH default→tofu), workflow_dispatch-only testacc.yml with cli choice input (tofu/terraform/both). Exit bar: local testacc green against BOTH tofu and terraform; dispatch workflow itself verifiable only post-merge (workflow_dispatch needs the file on the default branch).

## 2026-08-12 19:32 — Phase 3 complete, PR #9 open and green
Acceptance suite delivered: create/read, in-place rename with CompareValue id stability, ImportStateVerify, data source CompareValuePairs, store-backed CheckDestroy (fails on zero inspected). Key finding for the template: plugin-testing registers a legacy '-' namespace reattach address that OpenTofu hard-rejects at init; fix = TestMain sets TF_ACC_PROVIDER_NAMESPACE=meigma + explicit required_providers source. testacc task runInCI:false, dispatch-only workflow with cli choice matrix. terraform 1.15.8 pinned. I re-verified both CLI legs, root:check, and testacc absent from moon ci (0 matches). PR #9 green. Post-merge step: dispatch testacc.yml with cli=both to verify the workflow itself. Phase 6 note: DELETE_ME must mention acceptance tests break in generated repos until a real provider exists (by design).

## 2026-08-12 19:48 — Phase 3 merged; testacc dispatch failed; Phase 4 started
PR #9 merged (master 6466048). Post-merge dispatch of testacc.yml (cli=both) FAILED both legs: moon "No tasks found" — moon's CI detection filters runInCI:false tasks even for explicit `moon run` when CI env is set (GitHub always sets CI=true). Reproduced locally; CI='' bypasses and suite passes. Hotfix delegated to phase3-impl in fix/testacc-ci-detection worktree (step-level CI:'' env + comment). Meanwhile Phase 4 (docs pipeline) agent spawned in feat/docs-pipeline: registry+mkdocs single docs/ tree, tfplugindocs pin+gen+drift check, mkdocs.yml to repo root, Diátaxis skeleton, and researching the registry-extra-dirs open question with citations.

## 2026-08-12 20:05 — Phase 3 fully closed: testacc workflow green both legs
Hotfix PR #10 merged (master aabeda8): CI:'' step env bypasses moon's runInCI filtering (which applies to explicit moon run too), plus -count=1 defeating Go test cache silent-pass (agent proved with exec-logging wrapper: 40→0 invocations before, 40→40 after). Re-dispatched testacc cli=both: SUCCESS — tofu leg PASS (0.93s), terraform leg PASS (1.06s), real test output confirmed in logs. Phase 3 exit criteria fully met. Phase 6 reminder: CONTRIBUTING testacc section could mention the CI env gotcha for developers who export CI locally. Phase 4 (docs) agent still running.

## 2026-08-12 20:20 — Design's last open question resolved
Phase 4's delegated research closed it conclusively: both registries allowlist-ingest docs/ and ignore unknown subdirs. Evidence: tfplugindocs v0.25.0 source (walker comment: "valid non-documentation directories"), empirical validator run against a Diátaxis fixture (exit 0; extra dirs never enter the file list), and ~12 published providers with extra docs/ content incl. hashicorp/terraform-provider-tfe (docs/stylesheets/extra.css) and huaweicloud (4154 docs, disproving the dead 2000-file cap). Constraints recorded in DESIGN.md D3: no nesting inside recognized dirs (hard error), extra content is repo-only (registry-visible prose → docs/guides/ with page_title), never add website/docs/ (mixed-layout error + OpenTofu prefers it and would ignore docs/ entirely). tfplugindocs validate = CI guardrail. DESIGN.md updated; open questions now zero.
