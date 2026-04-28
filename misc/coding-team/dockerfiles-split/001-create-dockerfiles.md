# 001 - Create Dockerfiles for kafkarator, canary, canary-deployer

## Context
Project moving away from Earthly. Earthfile currently builds three images: `kafkarator`, `canary`, `canary-deployer`. This task replicates those builds as standalone Dockerfiles. Earthfile remains untouched in this iteration.

Refer to `Earthfile` at repo root for the source of truth on current build behavior.

## Objective
Add three Dockerfiles at the repo root that produce equivalent images to the Earthfile targets.

## Scope
Create the following files at repo root:

1. `Dockerfile.kafkarator`
2. `Dockerfile.canary`
3. `Dockerfile.canary-deployer`

### Dockerfile.kafkarator and Dockerfile.canary
- Multi-stage.
- Builder stage: `golang:1.25.7` (exact pin).
- Set `CGO_ENABLED=0`, `GOOS=linux`, `GOARCH=amd64`, `GO111MODULE=on` as ENV.
- Copy `go.mod` and `go.sum` first, run `go mod download`, then copy the rest of the source. This is the standard Docker layer-caching pattern equivalent to Earthfile's `dependencies` target.
- Build command mirrors Earthfile:
  - kafkarator: `go build -installsuffix cgo -o kafkarator cmd/kafkarator/*.go`
  - canary: `go build -installsuffix cgo -o canary cmd/canary/*.go`
- Final stage: `cgr.dev/chainguard/static:latest`, `WORKDIR /`, copy the binary to `/`, `CMD ["/kafkarator"]` or `CMD ["/canary"]`.

### Dockerfile.canary-deployer
- Base: `ghcr.io/nais/deploy/deploy:latest` (keep `:latest` as in Earthfile).
- Use a separate stage `FROM ghcr.io/astral-sh/uv:latest AS uv-provider` to source the `uv` binary, then `COPY --from=uv-provider /uv /usr/local/bin/` in the final stage (mirrors Earthfile's `uv-provider` target).
- `WORKDIR /canary/`
- `COPY canary-deployer/requirements.txt /canary/`
- `RUN apk add python3 && /usr/local/bin/uv pip install --system --requirement /canary/requirements.txt`
- `COPY canary-deployer/*.yaml /canary/`
- `COPY canary-deployer/deployer.py /canary/`
- `RUN python3 -c "import deployer"` (smoke-test import).
- `CMD ["python3", "/canary/deployer.py"]`

## Non-goals / Later
- Do NOT modify or delete `Earthfile`.
- Do NOT add `ARG VERSION`, OCI labels, or any image metadata.
- Do NOT add `.dockerignore` (separate concern; can be addressed later if needed).
- Do NOT change CI/CD pipelines or any other tooling.
- Do NOT introduce new dependencies.

## Constraints / Caveats
- Behavior must match Earthfile: same base images, same build flags, same entrypoints, same final layout.
- No code comments inside Dockerfiles unless syntactically required (project rule: code self-explanatory via naming).
