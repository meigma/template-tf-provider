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
