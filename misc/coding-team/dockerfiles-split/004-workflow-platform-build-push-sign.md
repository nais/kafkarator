# 004 - Switch GitHub workflow build job to nais/platform-build-push-sign

## Context
Tasks 001–003 introduced standalone Dockerfiles, a Docker Bake file, and verified bake-built images are functionally equivalent to Earthly-built images. Next step: retire Earthly from CI by switching the `build` job in `.github/workflows/main.yml` to the `nais/platform-build-push-sign` action.

Action reference: <https://github.com/nais/platform-build-push-sign>. Default registry is `europe-north1-docker.pkg.dev/nais-io/nais/images`. The action handles GCP auth, login, buildx, build, push, SBOM, and SLSA signing internally.

## Objective
Replace the Earthly-driven build/push/sign steps in the `build` job with a single `nais/platform-build-push-sign` invocation per matrix target, preserving current image registry paths and keeping the rest of the pipeline (version, test_validate, chart, rollout) functionally unchanged.

## Scope

Edit only `.github/workflows/main.yml`.

### Top-level `env` block
- Remove: `IMAGE_REGISTRY`, `EARTHLY_USE_INLINE_CACHE`, `EARTHLY_SAVE_INLINE_CACHE`, `EARTHLY_VERBOSE`, `EARTHLY_FULL_TARGET`, `EARTHLY_OUTPUT`.
- Keep: `GOOGLE_REGISTRY`, `FEATURE_REGISTRY` (still consumed by the `chart` job).

### `build` job
- Keep job-level: `name`, `runs-on`, `permissions` (`contents: read`, `id-token: write`), `needs: [test_validate, version]`, `strategy.matrix.target = [kafkarator, canary, canary-deployer]`, `fail-fast: false`, `env.VERSION`.
- Replace all current steps with:
  1. `actions/checkout` (keep existing pinned ref).
  2. `nais/platform-build-push-sign` invocation:
     - `name: kafkarator/${{ matrix.target }}` — this preserves current registry paths (`europe-north1-docker.pkg.dev/nais-io/nais/images/kafkarator/{kafkarator,canary,canary-deployer}`).
     - `dockerfile: Dockerfile.${{ matrix.target }}`
     - `google_service_account: gh-kafkarator`
     - `workload_identity_provider: ${{ secrets.NAIS_IO_WORKLOAD_IDENTITY_PROVIDER }}`
     - `push: ${{ github.ref == 'refs/heads/master' }}`
     - `extra_tags: ${{ needs.version.outputs.VERSION }}` — ensures the chart's pinned tag exists on the pushed image.
- Drop entirely (handled by the action or no longer relevant):
  - `sigstore/cosign-installer`
  - `cosign verify` of `gcr.io/distroless/static-debian11` (the runner image isn't used anymore — Dockerfiles now use `cgr.dev/chainguard/static`).
  - `google-github-actions/auth`
  - `docker/login-action`
  - `earthly/actions-setup` and the `earthly +docker-...` step.
  - The `docker pull` + `docker inspect` digest retrieval step.
  - `cosign sign`, `aquasecurity/trivy-action` SBOM, `cosign attest`.

### Action pinning
Pin `nais/platform-build-push-sign` to a commit SHA following the existing ratchet style used elsewhere in the file (e.g. `uses: nais/platform-build-push-sign@<sha> # ratchet:nais/platform-build-push-sign@<ref>`). Resolve the current default-branch SHA at implementation time. If a stable release ref/tag exists, prefer that as the human-readable ratchet target; otherwise use `main`.

### Untouched
- `version`, `test_validate`, `chart`, `rollout` jobs.
- `concurrency`, `on`, top-level `name`.
- `Earthfile`, all `Dockerfile.*`, `docker-bake.hcl`, `.mise-tasks/`, `charts/`.

## Non-goals / Later
- Do NOT delete `Earthfile` in this task.
- Do NOT change chart `values.yaml`, since `name: kafkarator/<target>` preserves current image paths.
- Do NOT modify the `chart` or `rollout` jobs.
- Do NOT add multi-arch builds.
- Do NOT introduce additional dependencies.

## Constraints / Caveats
- Image paths must remain `europe-north1-docker.pkg.dev/nais-io/nais/images/kafkarator/{kafkarator,canary,canary-deployer}` — the `chart` job and Helm values depend on this.
- The `VERSION` produced by the `version` job must continue to be a valid tag on every pushed image (delivered via `extra_tags`).
- Preserve `permissions: { contents: read, id-token: write }` on the `build` job — required for keyless signing and workload identity.
- Action handles auth only when `push == 'true'`; PR builds (non-master) will build without pushing, which is the intended behavior.
- Keep ratchet-style pinning consistent with the rest of the file.
