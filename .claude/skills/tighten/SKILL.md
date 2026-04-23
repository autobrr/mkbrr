---
name: tighten
description: Iterative adversarial review loop for shipping implementation work. Use after any implementation is complete — when tests pass and the code is ready for scrutiny. Triggers on phrases like "tighten", "review loop", "preflight", "ship this", or when implementation work has just been completed and needs hardening before commit. Requires the Codex plugin for adversarial reviews.
---

# Tighten

Harden implementation work through iterative adversarial review, then ship it.

## Why this exists

Code reviews catch bugs that tests miss — precedence conflicts, silent coercion, schema drift, API contract violations. Running multiple adversarial review passes with triage between each pass surfaces these issues before users do. The key insight: not every finding is valid. Triaging findings (accepting real bugs, pushing back on false positives) is where the value lives.

## Prerequisites

This skill requires the Codex plugin for adversarial reviews. If `/codex:adversarial-review` is not available, stop and tell the user:

```
The tighten skill requires the Codex plugin. Install it:
  /plugin marketplace add openai/codex-plugin-cc
  /plugin install codex@openai-codex
  /reload-plugins
  /codex:setup
```

## The Loop

### 1. Verify readiness

Before entering the review loop, confirm:
- All tests pass (`make test`)
- Code builds cleanly (`go build ./...`)
- Lint has no new issues from your changes (`make lint` — pre-existing issues are fine)

If any of these fail, fix them first. Don't waste review cycles on code that doesn't compile.

### 2. Run adversarial review

Ask the user to run `/codex:adversarial-review --wait` (this skill has `disable-model-invocation` so you cannot invoke it yourself). This reviews the working tree diff (all uncommitted changes against the current branch). If you need to review a branch diff against main instead, ask for `/codex:adversarial-review --base main --wait`.

### 3. Triage findings

This is the critical step. For each finding, make a judgment call:

**Auto-fix** (don't ask the user):
- Missing validation that exists in parallel code paths (consistency bugs)
- Schema/config drift (a new field was added in code but not in JSON schemas, examples, etc.)
- Zero/nil guards that are clearly missing
- Off-by-one or boundary errors with obvious fixes

**Escalate to user** (present your assessment, let them decide):
- Design disagreements — where the reviewer questions the approach but you believe the approach is intentional
- Tradeoff decisions — where fixing the finding has costs (complexity, performance, scope creep)
- Findings that are technically correct but arguably not worth fixing

**Present triage as a concise table:**
```
| # | Severity | Finding | Verdict | Reason |
```

Verdict is one of: `fix`, `disagree`, `ask user`.

### 4. Apply fixes

- `fix` verdicts: apply immediately
- `ask user` verdicts: present your assessment, wait for their call
- `disagree` verdicts: skip — these are intentional design choices, note them so you can defend them if the next review pass raises them again

After fixing, re-run tests and build to make sure fixes don't break anything.

### 5. Re-review

Ask the user to run `/codex:adversarial-review --wait` again. Repeat from step 3 until:
- The review comes back clean (verdict: `looks-good` or only low/informational findings)
- All remaining findings have been triaged as intentional disagreements
- Two consecutive reviews surface no new high-severity findings

Typically 2-3 passes is enough. If you're past 4 passes, something structural is wrong — stop and discuss with the user.

### 6. Smoke test on real data

Don't just rely on unit tests. Create temporary real-ish content and exercise the feature end-to-end:
- Happy path with realistic inputs
- Edge cases and error paths
- Verify output makes sense (inspect it, don't just check exit codes)

Capture the smoke test results — they're useful for the PR description. Clean up temp files when done.

### 7. Ship it

Commit and create a PR per project conventions (see CLAUDE.md for commit format and PR body guidelines). Include smoke test results in the PR if they demonstrate the feature well.

### 8. Check for docs impact

As a final step, consider: does this change affect user-facing behavior? New CLI flags, config fields, or changed defaults all count. If so, nudge the user:

> This adds user-facing changes — specifically [list what changed]. Consider running `/docs` to check if mkbrr.com needs updating.

Be specific about what changed so the user can make a quick call. Don't automatically create the docs PR from this skill — that's the `docs` skill's job.

## When to stop

The goal is not zero findings. The goal is zero *valid unaddressed* findings. Reviewers will always find something to say — knowing when to stop is part of the skill.
