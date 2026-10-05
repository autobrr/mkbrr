---
name: docs
description: Find out if the mkbrr.com docs in documentation/ need an update, and edit them in the current branch. Two modes: targeted (on a feature branch, sync specific changes) and sweep (audit entire docs site against codebase for drift). Triggers on "/docs", "update docs", "docs check", "docs sweep", "docs audit", or after shipping features that change how users interact with mkbrr.
---

# Docs Sync

Keep the mkbrr.com documentation site in `documentation/` in sync with the mkbrr codebase. Docs edits go in the same branch and PR as the code change.

## Two modes

**Targeted mode** — on a feature branch, sync specific changes from that branch to docs.
**Sweep mode** — on `develop` (or anytime), audit the entire docs site against the current codebase for drift, missing info, or contradictions.

Pick the mode based on context:
- Feature branch with uncommitted/recent work → targeted
- On `develop`, or user says "sweep"/"audit" → sweep
- After a release → sweep

## Drift surface

Everything in the docs that can fall out of sync with the codebase. All docs paths are relative to `documentation/`.

| Area | Docs file(s) | Codebase source of truth |
|------|-------------|-------------------------|
| CLI create flags | `snippets/create-params.mdx` + shared `snippets/common-*.mdx` | `cmd/create.go` flag definitions in `init()` |
| CLI modify flags | `snippets/modify-params.mdx` + shared `snippets/common-*.mdx` | `cmd/modify.go` flag definitions |
| CLI check flags | `snippets/check-params.mdx` + shared `snippets/common-*.mdx` | `cmd/check.go` flag definitions |
| CLI reference pages | `cli-reference/create.mdx`, `modify.mdx`, `check.mdx` | Same as above (examples, usage text can drift independently from param snippets) |
| Quickstart examples | `quickstart.mdx` | Flag names, value ranges, and defaults in `cmd/create.go` |
| Preset config fields | `features/presets.mdx` | `internal/preset/preset.go` `Options` struct |
| Batch config fields | `features/batch-mode.mdx` | `torrent/batch.go` `BatchJob` struct |
| Tracker rules table | `features/tracker-rules.mdx` | `internal/trackers/trackers.go` `trackerConfigs` slice |
| Filtering defaults | `features/filtering.mdx` | Default exclude patterns in `torrent/create.go` and `torrent/ignore.go` |
| Season pack detection | `features/season-packs.mdx` | `torrent/seasonfinder.go` regex patterns |
| Piece size algorithm | `guides/creating-torrents.mdx`, `quickstart.mdx` | `torrent/create.go` `calculatePieceLength()` size tiers |
| Modify capabilities | `guides/modifying-torrents.mdx`, `cli-reference/modify.mdx` | `torrent/modify.go` |
| JSON schemas | Referenced in preset/batch docs | `schema/presets.json`, `schema/batch.json` |
| Development commands | `development.mdx` | `Makefile` targets |
| Installation methods | `installation.mdx` | Release artifacts, Dockerfile, package configs |

For Mintlify syntax, components, and `docs.json` settings, use the `mintlify` skill.

Note: the `snippets/common-*.mdx` files (e.g., `common-private.mdx`, `common-entropy.mdx`) are shared across multiple commands. A flag may be documented there rather than in the main params file — check both.

## Targeted mode

Use when on a feature branch with specific changes to sync.

### 1. Identify what changed

Look at the branch diff, including uncommitted changes:

```bash
git diff "$(git merge-base develop HEAD)" --stat
git status --short
```

Identify user-facing changes using the drift surface table. If nothing is user-facing, tell the user "No docs update needed" and stop.

If the branch diff already changes the affected docs pages, review them for completeness. If complete, tell the user and stop.

### 2. Find affected docs pages and update

For each user-facing change, use the drift surface table to find the docs file(s). Read them, then make targeted edits. Match the surrounding style and keep changes minimal.

### 3. Ship it

Commit the docs edits on the current feature branch, so they ship in the same PR as the code.

## Sweep mode

Use to audit the full docs site against the current codebase.

### 1. Walk the drift surface

For each row in the drift surface table, compare the docs against the codebase source of truth:

- Read the codebase source (e.g., `cmd/create.go` flag defs, `trackerConfigs` slice)
- Read the corresponding docs file(s)
- Compare: are there flags/fields/trackers/patterns in the code that aren't in the docs? Are there things in the docs that no longer exist in the code? Are flag names, defaults, and value ranges accurate?

When the drift surface has many rows, parallelize by dispatching independent comparisons as subagents where possible.

### 2. Read reader feedback

If `MINTLIFY_API_KEY` and `MINTLIFY_PROJECT_ID` are not set, skip this step. Otherwise, run `scripts/docs-feedback.sh`. It covers the last 30 days, or the number of days that you pass. It lists the downvotes and comments for each page, and the searches where no reader clicked a result.

Read each page that has a downvote or a comment, and add each real problem to the findings. A search with no clicks can point to a topic that the docs do not cover. This step is done when each listed page and search is a finding or is dismissed with a reason.

### 3. Report findings

Present a table of discrepancies:

```
| Area | Issue | Docs file | Code file | Type |
```

Type is one of:
- **Missing** — exists in code but not in docs
- **Stale** — exists in docs but removed/changed in code
- **Inaccurate** — docs describe it wrong (wrong flag name, wrong default, wrong value range)

If nothing is found, tell the user "Docs are in sync" and stop.

### 4. Fix and PR

Group related fixes into a single PR. Create a branch from `develop` (for example `docs/sweep`), commit, push, and open the PR against `develop`. If the drift is on the live site and the fix must go out before the next release, open the PR against `main` instead, then merge `main` back into `develop`.
