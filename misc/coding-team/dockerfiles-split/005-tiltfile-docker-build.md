# 005 - Switch Tiltfile from earthly to docker_build

## Context
Local dev loop currently invokes Earthly via Tilt's `custom_build`. With Dockerfiles now at the repo root (task 001), the Tiltfile should use Tilt's native `docker_build` against `Dockerfile.kafkarator`.

Only the kafkarator image participates in the Tilt loop; canary and canary-deployer are not loaded by Tilt.

## Objective
Replace the Earthly-driven `custom_build` block in `Tiltfile` with a `docker_build` that uses `Dockerfile.kafkarator`.

## Scope
Edit only `Tiltfile`.

Replace this block:

```
ignore = str(read_file(".earthignore")).split("\n")
custom_build(
    ref=APP_NAME,
    command="earthly +docker-kafkarator --VERSION=$EXPECTED_TAG --REGISTRY=$EXPECTED_REGISTRY",
    deps=["cmd", "controllers", "pkg", "go.mod", "go.sum", "Earthfile"],
    skips_local_docker=False,
    ignore=ignore,
)
```

With:

```
docker_build(
    ref=APP_NAME,
    context=".",
    dockerfile="Dockerfile.kafkarator",
    only=["cmd", "controllers", "pkg", "go.mod", "go.sum", "Dockerfile.kafkarator"],
)
```

Drop the `ignore = str(read_file(".earthignore")).split("\n")` line entirely. Tilt's `docker_build` automatically honors `.dockerignore` for the build context.

## Untouched
- All other lines in `Tiltfile` (helm_resource, k8s_yaml, k8s_resource, etc.).
- `Earthfile`, `.earthignore`, `.dockerignore`.
- All other repo files.

## Non-goals / Later
- Do NOT delete `Earthfile` or `.earthignore` in this task.
- Do NOT add live_update / fast rebuild support.
- Do NOT change Helm chart wiring or k8s_yaml/k8s_resource calls.
- Do NOT modify Dockerfiles.

## Constraints / Caveats
- `Dockerfile.kafkarator` must be present at repo root (it is, from task 001).
- `only` includes `Dockerfile.kafkarator` so edits to it trigger rebuilds; the previous `deps` listed `Earthfile`, which no longer applies.
