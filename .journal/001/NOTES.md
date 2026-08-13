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
