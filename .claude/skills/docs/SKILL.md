---
name: docs
description: Check if mkbrr.com docs need updating and create a PR if so. Two modes — targeted (on a feature branch, sync specific changes) and sweep (on main, audit entire docs site against codebase for drift). Triggers on "/docs", "update docs", "docs check", "docs sweep", "docs audit", or after shipping features that change how users interact with mkbrr.
---

# Docs Sync

Keep the mkbrr.com documentation site in sync with the mkbrr codebase.

## Repos

- **mkbrr** (code): `/Users/soup/github/autobrr/mkbrr` (repo: `autobrr/mkbrr`)
- **mkbrr.com** (docs): `/Users/soup/github/soup/mkbrr.com` (repo: `s0up4200/mkbrr.com`)

## Two modes

**Targeted mode** — on a feature branch, sync specific changes from that branch to docs.
**Sweep mode** — on main (or anytime), audit the entire docs site against the current codebase for drift, missing info, or contradictions.

Pick the mode based on context:
- Feature branch with uncommitted/recent work → targeted
- On main, or user says "sweep"/"audit" → sweep
- After a release → sweep

## Drift surface

Everything in the docs that can fall out of sync with the codebase. All docs paths are relative to the docs repo root.

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

Note: the `snippets/common-*.mdx` files (e.g., `common-private.mdx`, `common-entropy.mdx`) are shared across multiple commands. A flag may be documented there rather than in the main params file — check both.

## Targeted mode

Use when on a feature branch with specific changes to sync.

### 1. Identify what changed

Work from the mkbrr repo at `/Users/soup/github/autobrr/mkbrr`. Look at the branch diff:

```bash
git diff main...HEAD --stat
```

Identify user-facing changes using the drift surface table. If nothing is user-facing, tell the user "No docs update needed" and stop.

### 2. Check for existing docs work

Before creating anything:

```bash
gh pr list --repo s0up4200/mkbrr.com --state open
cd /Users/soup/github/soup/mkbrr.com && git branch --list 'docs/*'
```

If a docs PR/branch already exists for this feature, review it for completeness. If complete, tell the user and stop. If incomplete, update the existing branch.

### 3. Find affected docs pages and update

For each user-facing change, use the drift surface table to find the docs file(s). Read them in the docs repo, then make targeted edits — match surrounding style, keep changes minimal.

### 4. Ship it

In the docs repo:
- Create a branch from main (e.g., `docs/feature-name`)
- Commit with a descriptive message
- Push and create a PR on `s0up4200/mkbrr.com`
- Reference the mkbrr PR in the body
- Comment on the mkbrr PR with a link to the docs PR

## Sweep mode

Use to audit the full docs site against the current codebase.

### 1. Walk the drift surface

For each row in the drift surface table, compare the docs against the codebase source of truth:

- Read the codebase source (e.g., `cmd/create.go` flag defs, `trackerConfigs` slice)
- Read the corresponding docs file(s)
- Compare: are there flags/fields/trackers/patterns in the code that aren't in the docs? Are there things in the docs that no longer exist in the code? Are flag names, defaults, and value ranges accurate?

When the drift surface has many rows, parallelize by dispatching independent comparisons as subagents where possible.

### 2. Report findings

Present a table of discrepancies:

```
| Area | Issue | Docs file | Code file | Type |
```

Type is one of:
- **Missing** — exists in code but not in docs
- **Stale** — exists in docs but removed/changed in code
- **Inaccurate** — docs describe it wrong (wrong flag name, wrong default, wrong value range)

If nothing is found, tell the user "Docs are in sync" and stop.

### 3. Fix and PR

Group related fixes into a single PR. Follow the same ship process as targeted mode — branch, commit, push, PR.
