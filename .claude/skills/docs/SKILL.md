---
name: docs
description: Check if mkbrr.com docs need updating and create a PR if so. Use when shipping user-facing features, adding CLI flags, changing config formats, or modifying preset/batch behavior. Triggers on "/docs", "update docs", "docs check", "docs sync", or after shipping features that change how users interact with mkbrr.
---

# Docs Sync

Check whether the mkbrr.com documentation site needs updating for recent changes, and create a PR if it does.

## Why this exists

mkbrr has a separate docs site (repo: `s0up4200/mkbrr.com`, local: `/Users/soup/github/soup/mkbrr.com`). When user-facing features ship in the main repo (`/Users/soup/github/autobrr/mkbrr`), the docs site needs to stay in sync.

## When docs need updating

**Yes:**
- New CLI flags or flag behavior changes
- New or modified preset config fields
- New or modified batch config fields
- Changes to tracker rules or constraints
- New commands or subcommands
- Changes to default behavior

**No:**
- Bug fixes that don't change user-facing behavior
- Internal refactors, test-only changes, performance improvements

## The docs site structure

The docs site uses Mintlify (MDX files):

| What | Where |
|------|-------|
| CLI create flags | `snippets/create-params.mdx` (imported by `cli-reference/create.mdx` and `guides/creating-torrents.mdx`) |
| Preset config reference | `features/presets.mdx` |
| Batch config reference | `features/batch-mode.mdx` |
| Tracker rules | `features/tracker-rules.mdx` |
| Guides | `guides/` |

## Process

### 1. Identify what changed

Work from the mkbrr repo at `/Users/soup/github/autobrr/mkbrr`. Look at the current branch's diff against main:

```bash
git diff main...HEAD --stat
```

Identify user-facing changes — new flags, config fields, behavior changes. If nothing is user-facing, tell the user "No docs update needed" and stop.

### 2. Check for existing docs work

Before creating anything, check if a docs PR already exists for this feature:

```bash
gh pr list --repo s0up4200/mkbrr.com --state open
```

Also check if a docs branch already exists locally:

```bash
cd /Users/soup/github/soup/mkbrr.com && git branch --list 'docs/*'
```

If docs work already exists, review it for completeness against the changes identified in step 1. If it's complete, tell the user and stop. If it's incomplete, update the existing branch rather than creating a new one.

### 3. Find affected docs pages

For each user-facing change, determine which docs files need updating using the table above. Read those files in the docs repo to understand the current style and structure.

### 4. Create a branch and make changes

In the docs repo (`/Users/soup/github/soup/mkbrr.com`):
- Create a branch from main (e.g., `docs/feature-name`)
- Make targeted edits — add new fields, flags, or sections alongside existing ones
- Match the style and structure of surrounding content
- Keep changes minimal — don't reorganize or rewrite existing docs

### 5. Commit, push, and PR

- Commit with a descriptive message (e.g., `docs: add target_piece_count option`)
- Push and create a PR on `s0up4200/mkbrr.com`
- Reference the mkbrr PR in the body (e.g., `Documents changes from autobrr/mkbrr#156`)
- Keep the PR body short

### 6. Link back

Comment on the mkbrr PR with a link to the docs PR:

```bash
gh pr comment <mkbrr-pr-number> --repo autobrr/mkbrr --body "Docs PR: <docs-pr-url>"
```
