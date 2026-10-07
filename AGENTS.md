# AGENTS.md

mkbrr creates, inspects, checks, and modifies torrent files. It has a CLI, a GUI, and a batch mode. `GLOSSARY.md` defines the domain terms.

## Layout

- `main.go` calls `cmd/`, which holds one Cobra command per file.
- `torrent/` is the public Go package with the torrent logic: create, hash, verify, modify, batch, and season pack detection. Go programs outside this repo import it, so a change to an exported name is a breaking change.
- `internal/trackers/trackers.go` holds the tracker rules in the `trackerConfigs` slice. `internal/preset/` loads `presets.yaml`.
- `gui/` is the Wails GUI. It is a separate Go module with its own `go.mod` and a frontend in `gui/frontend/`.
- `schema/` holds the JSON schemas for `presets.yaml` and `batch.yaml`. `examples/` holds sample files for both.
- `documentation/` is the mkbrr.com docs site (Mintlify). `docs/` holds the ADRs and the agent docs.

## Commands

- `make build` writes the binary to `build/mkbrr`.
- `make test` runs all tests. `make test-race` runs `./torrent` with the race detector, as CI does.
- `make test-large` runs the `large_tests` build tag. These tests hash large files and use all CPU cores.
- `make lint` runs `golangci-lint`.
- `make gui-dev` starts the GUI in Wails dev mode.

## Settings

The CLI, the GUI, and batch mode share one settings type for each operation: `torrent.CreateSettings` and `torrent.ModifySettings` (see `docs/adr/0001-shared-settings-types.md`). When you add a setting, add it to the shared type, and then to each place that `cmd/settings_parity_test.go` requires. A preset key or a batch key also needs an entry in `schema/*.json`.

## Branches and docs site

- `develop` is the default branch and the base branch for feature PRs.
- `main` holds the latest release. Mintlify builds the mkbrr.com site in `documentation/` from `main`, so the live site describes the latest release.
- mkbrr.com uses the Mintlify free plan. Admin keys, assistant keys, and the analytics API are not available.
- Update `documentation/` in the same PR as a change that users can see. The `docs` skill maps code to docs pages.
- Load the `mintlify` skill before you change a file in `documentation/`. It covers Mintlify pages, navigation, and components.
- A fix to the live docs goes as a PR to `main`. Then merge `main` back into `develop`.
- A release has three steps, in this order:
  1. Fast-forward `main` to `develop` (`git push origin origin/develop:main`). Then tag that commit and push the tag.
  2. The tag starts the changelog workflow, which opens a PR against `main`. Merge that PR.
  3. Merge `main` back into `develop`. If you skip this step, the fast-forward at the next release fails.
- See `docs/adr/0002-develop-main-and-docs-site.md`.

## Commits and PRs

- Use Conventional Commits: `type(scope): description`, for example `fix(torrent): correct piece length for small files`.
- PRs are squash-merged, so the PR title becomes the commit subject. A breaking change has `!` in the title (`feat(torrent)!: ...`) and the `BREAKING CHANGE` label. The `PR title` workflow enforces this, and goreleaser lists these commits under Breaking Changes.
- Do not mention Claude or Claude Code in commit messages.
- Do not commit real torrent payloads, private tracker URLs, or torrent names.

## Agent skills

### Issue tracker

Issues are tracked in GitHub Issues for autobrr/mkbrr, through the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

The default label names are used: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`, `needs-grilling`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `GLOSSARY.md` and `docs/adr/` at the repo root. See `docs/agents/domain.md`.

## Code Review Rules

These rules are for AI PR reviewers. The agent workflow rules in this file are for coding agents. Do not apply them to PR authors.

- Report a defect only when the change causes a concrete wrong behavior. Name the trigger and the result for the user. If you cannot name both, omit the finding.
- Check the merge base. If `develop` already has the problem, still report it, but label it "already on develop" and do not call it a regression.
- When the PR body, a linked issue, an ADR in `docs/adr/`, or a code comment calls a behavior deliberate, respond to that reason. Report a design flaw only when you can say why the stated reason does not hold.
- Do not report what gofmt or golangci-lint already report. Do not ask for docstrings.
- Read earlier review threads. Do not repeat a finding that was resolved or refuted, unless you have new evidence.
- Treat a change that gives a different info hash for the same files and settings as P1, unless the PR body names it under "Behaviour change". Users cross-seed with these torrents, so a new hash breaks their existing seeds.
- Treat a change to an exported name in `torrent/` as P1 when the PR title has no `!`. Go programs outside this repo import the package.
