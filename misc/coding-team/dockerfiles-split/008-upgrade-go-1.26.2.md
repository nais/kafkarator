# 008 - Upgrade Go to 1.26.2

## Context
Project currently pins Go 1.25.7 in four places:
- `mise.toml` — `go = "1.25.7"`
- `go.mod` — `go 1.25.7`
- `Dockerfile.kafkarator` — `FROM golang:1.25.7 AS builder`
- `Dockerfile.canary` — `FROM golang:1.25.7 AS builder`

Latest stable is Go 1.26.2.

## Objective
Bump the Go toolchain to 1.26.2 in all four locations. No other changes.

## Scope

### `mise.toml`
- Change `go = "1.25.7"` to `go = "1.26.2"`.

### `go.mod`
- Replace the existing `go 1.25.7` directive with two lines using the toolchain-style split:
  ```
  go 1.26
  
  toolchain go1.26.2
  ```
- The blank line is intentional; `go mod tidy` may rewrite formatting — that's fine.

### `Dockerfile.kafkarator` and `Dockerfile.canary`
- Change `FROM golang:1.25.7 AS builder` to `FROM golang:1.26.2 AS builder`.

### Verification
After making the edits, run:
- `mise run check`
- `mise run test`

Both must succeed. If either fails for reasons related to the Go upgrade, report back rather than papering over.

## Untouched
- All Go source files.
- All `require` directives in `go.mod` and `go.sum`.
- `Dockerfile.canary-deployer`, `docker-bake.hcl`, `Tiltfile`, workflows, charts, mise tasks.

## Non-goals / Later
- Do NOT run `go get -u ./...` or otherwise update dependencies.
- Do NOT modify source code to use 1.26 features.
- Do NOT touch `Dockerfile.canary-deployer` (Python image, unaffected).

## Constraints / Caveats
- Use exact version `1.26.2` in mise and Dockerfiles.
- `go.mod` must use the `go 1.26` + `toolchain go1.26.2` split form, not `go 1.26.2`.
- If `mise install` is needed to fetch the new Go, do that as part of the workflow.
