# Develop and main branches, and the docs site publishes from main

## Status

Accepted

## Context

The mkbrr.com docs site was in a separate repository, `s0up4200/mkbrr.com`. Each change that users can see needed a second PR there. Docs for new features waited in open PRs until a release shipped. After each release, a cross-repository dispatch with its own personal access token started the changelog update (#234).

## Decision

The mkbrr.com docs site lives in `documentation/`, so a feature and its docs ship in one PR. Mintlify builds the site from `main`. `main` moves only at a release, when it fast-forwards to `develop` (`git push origin origin/develop:main`). So the site always describes the released version.

Feature PRs go to `develop`, which is the default branch. A fix to the live docs is a PR to `main`. Then `main` is merged back into `develop` before the next fast-forward. The changelog entry follows the same path. After each release, the changelog workflow opens a PR against `main`. This is the same model as qui.

## Alternatives

- **One `main` branch that Mintlify builds from.** Docs for unreleased features go live up to three weeks before the release.
- **A `docs-live` branch that is reset to each release tag.** The reset at the next tag discards each direct fix to the live docs.

## Consequences

- Contributors open PRs against `develop`, not `main`.
- A PR that changes only `documentation/` does not start the release workflow.
- The dispatch job and the `DOCS_DISPATCH_TOKEN` secret are not necessary.
