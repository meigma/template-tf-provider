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
