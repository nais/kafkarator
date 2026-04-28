# Remove Makefile

## Context
`Makefile` only delegates to `mise run <target>`. All targets (`all`, `build`, `test`, `fmt`, `check`, `generate`) already have equivalent mise tasks under `.mise-tasks/`. No CI, Earthfile, Tiltfile, or scripts reference `make`/`Makefile`.

## Objective
Eliminate Makefile and any remaining references to `make` in the repo.

## Scope
1. Delete `Makefile`.
2. `README.md` (around lines 125-126): replace
   - `make kafkarator` → `mise run build:kafkarator`
   - `make canary` → `mise run build:canary`
   (Note: those Makefile targets never existed; the README was wrong. Equivalent mise tasks `.mise-tasks/build/kafkarator` and `.mise-tasks/build/canary` do exist.)
3. `flake.nix`: remove `gnumake` from the devShell `buildInputs` list.

## Non-goals / Later
- Do not modify any existing mise task scripts.
- Do not add new mise tasks.
- Do not touch unrelated `make(...)` occurrences in Go source, ADR docs, Helm `Chart.yaml` comments, or `.earthignore` (`cmake-build-*`).

## Constraints / Caveats
- Keep `flake.nix` formatting consistent with surrounding lines.
