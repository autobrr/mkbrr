# The GUI shows settings that Go resolves

## Status

Accepted

## Context

The GUI pages copied preset values into the form and repeated Go rules in TypeScript, and the copies drifted (#215, #270, #271, #272, #282, and the two GUI bugs of #281).

## Decision

The GUI sends only the values that the user typed, and a Go binding returns each resolved value and its origin. Pages show the result and don't hold a create or modify rule.

## Consequences

- Each form change costs a call to Go.
- A new rule goes only in Go.
- ADR 0001 still stands, and this extends it to the values that the GUI shows.
