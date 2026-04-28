# 006 - Remove all Earthly artifacts

## Context
Earthly has been fully replaced: Dockerfiles + `docker-bake.hcl` (tasks 001–002) handle image builds, the GitHub workflow uses `nais/platform-build-push-sign` (task 004), and `Tiltfile` uses `docker_build` (task 005). Image equivalence has been verified (task 003). Earthly artifacts can now be removed.

## Objective
Delete all Earthly-related files and remove Earthly references from `README.md`.

## Scope

### Files to delete
- `Earthfile`
- `.earthignore`
- `earthlyw`

### `README.md` edits

1. **Prerequisites** (around line 96): replace
   ```
   - [Earthly](https://earthly.dev) (for reproducible builds)
   ```
   with
   ```
   - [Docker](https://www.docker.com/) (with buildx, for building images)
   ```

2. **Build Docker images** subsection (around lines 102–106): replace the entire block
   ```
   - **Build Docker images:**
     ```sh
     ./earthlyw +docker
     ```
     This uses the `earthlyw` wrapper to ensure you always use the correct Earthly version for reproducible builds. It builds Docker images for both `kafkarator` and `canary`.
   ```
   with
   ```
   - **Build Docker images:**
     ```sh
     mise run docker
     ```
     This runs `docker buildx bake` via the mise task and builds all three images (`kafkarator`, `canary`, `canary-deployer`). To build a single image, use `mise run docker:kafkarator`, `mise run docker:canary`, or `mise run docker:canary-deployer`.
   ```

3. **Links** section (around line 177): remove the line
   ```
   - [Earthly](https://earthly.dev)
   ```

## Untouched
- Historical task briefs under `misc/coding-team/` — intentional historical record.
- `.dockerignore`, Dockerfiles, `docker-bake.hcl`, `.mise-tasks/`, `Tiltfile`, `charts/`, `.github/workflows/`, all source code.

## Non-goals / Later
- Do NOT edit historical task briefs (001–005).
- Do NOT modify any other documentation references that mention "earthly" only in historical/descriptive contexts outside `README.md`.
- Do NOT add new prerequisites beyond replacing the Earthly bullet.

## Constraints / Caveats
- Verify no remaining repository file (outside `misc/coding-team/` and the deleted files themselves) references `earthly`, `Earthfile`, `earthlyw`, or `.earthignore` after the change.
