# 002 - Add docker-bake.hcl and mise docker tasks

## Context
Task 001 added `Dockerfile.kafkarator`, `Dockerfile.canary`, `Dockerfile.canary-deployer` at the repo root. This task wires them up via Docker Bake and exposes mise tasks for invocation.

Earthfile push behavior baseline (from `Earthfile`):
- Registry: `europe-north1-docker.pkg.dev/nais-io/nais/images/kafkarator`
- Image names: `<registry>/kafkarator`, `<registry>/canary`, `<registry>/canary-deployer`
- Tags pushed: `:${VERSION}` and `:latest`

Existing mise task convention (see `.mise-tasks/build/`): each task is a shell script using `#!/usr/bin/env sh`, `set -e`, and a `#MISE description="..."` header line. A task namespace is a directory; `_default` is the entrypoint when invoking the namespace name itself; sibling files become `<namespace>:<name>` tasks.

## Objective
Provide a single Docker Bake entrypoint that can build all three images or any one individually, and surface those builds as mise tasks under a `docker:` namespace.

## Scope

### 1. `docker-bake.hcl` at repo root

- Declare two variables with defaults:
  - `VERSION` defaulting to `"latest"`
  - `REGISTRY` defaulting to `"europe-north1-docker.pkg.dev/nais-io/nais/images/kafkarator"`
- Declare `group "default"` containing all three targets.
- Declare three targets: `kafkarator`, `canary`, `canary-deployer`. Each target sets:
  - `context = "."`
  - `dockerfile = "Dockerfile.<name>"` (matching the file created in task 001)
  - `platforms = ["linux/amd64"]`
  - `tags = ["${REGISTRY}/<name>:${VERSION}", "${REGISTRY}/<name>:latest"]`

No `output`/`push` baked in — push is controlled by the caller via `--push`.

### 2. `.mise-tasks/docker/` directory

Create four shell scripts, each marked executable, following the existing pattern in `.mise-tasks/build/`:

- `.mise-tasks/docker/_default` — description "Build all docker images"; runs `docker buildx bake`.
- `.mise-tasks/docker/kafkarator` — description "Build kafkarator docker image"; runs `docker buildx bake kafkarator`.
- `.mise-tasks/docker/canary` — description "Build canary docker image"; runs `docker buildx bake canary`.
- `.mise-tasks/docker/canary-deployer` — description "Build canary-deployer docker image"; runs `docker buildx bake canary-deployer`.

Each script: `#!/usr/bin/env sh`, `#MISE description="..."`, `set -e`, then the bake command.

## Non-goals / Later
- Do NOT modify or delete `Earthfile`.
- Do NOT modify the existing Dockerfiles.
- Do NOT add `--push` or `--load` to the mise tasks.
- Do NOT add multi-arch platforms.
- Do NOT add `docker`/`buildx` to `mise.toml [tools]` — assumed system-installed.
- Do NOT change CI/CD.

## Constraints / Caveats
- Task scripts must mirror existing `.mise-tasks/build/*` style exactly (shebang, header comment, `set -e`).
- Ensure new task scripts are executable (`chmod +x`).
- No comments in the bake file unless syntactically required.
