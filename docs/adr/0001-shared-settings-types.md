# Shared settings types for the CLI, the GUI, and batch

## Status

Accepted

## Context

The CLI, the GUI, and batch mode each declared their own copy of the create and modify settings. The copies drifted apart. The GUI request had no field to clear a source, a comment, or the private flag, and the GUI ignored some preset values. Modify also had several ways to clear a setting: "was set" booleans for source and comment, a remove-private boolean, no-date, and no-creator.

## Decision

One exported type holds the settings for each operation. `torrent.ModifySettings` holds the modify settings. The CLI builds it from its flags, and the GUI modify request embeds it, so the GUI does not declare its own copies of the fields. The create settings get one type in the same way, and the GUI create request and the batch job embed it (#219).

Each modify setting is **keep**, **set**, or **clear** (see `GLOSSARY.md`). The zero value keeps. A value sets. An empty comment or source, or the `No` field of a setting, clears. Only an override can clear. A check for settings that conflict, such as `Entropy` together with `NoEntropy`, is in the torrent package, so that every caller gets it.

A parity test (#221) reads the shared types with reflection. It fails when a setting has no CLI flag, no batch or preset key where it must have one, or no GUI control. The test keeps explicit lists of exceptions and of known gaps.

## Alternatives

- **Flat fields with a parity test only.** Each caller keeps its own fields, and a test compares them. The test finds drift, but each caller still maps every field by hand, and the GUI and the CLI can still give a field different meanings.
- **Frontend tests.** Tests in the GUI frontend check that each control sends the correct field. These tests do not see the CLI or batch, so they cannot find drift between callers.

## Consequences

- The exported modify options changed. This is a breaking change for Go importers.
- The GUI request can now clear a setting. The GUI modify page gets the controls for it later (#222).
- A new setting goes in one place, and every caller gets it. The parity test shows where a caller has no control for it yet.
