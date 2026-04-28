# 007 - Make canary-deployer a proper Python project with uv

## Context
`canary-deployer` is currently an ad-hoc Python script (`deployer.py`) with a flat `requirements.txt`. The image is built by `Dockerfile.canary-deployer` using `uv pip install --system -r requirements.txt`. Make it a proper uv-managed Python project (flat layout, hatchling build backend, locked dependencies).

Current dependencies (`canary-deployer/requirements.txt`):
- `trio==0.31.0`
- `pydantic==2.12.5`
- `pydantic-settings==2.11.0`

`__pycache__` is already in the global `.gitignore`, but `canary-deployer/__pycache__/` is currently tracked — it must be removed from version control.

`mise.toml` currently has only `go` and `helm` under `[tools]`.

## Objective
Convert `canary-deployer` to a uv-managed Python project (flat layout) and update the Dockerfile to use `uv sync --frozen`. Add `python` and `uv` to mise tools.

## Scope

### 1. `canary-deployer/pyproject.toml` (new)

- `[project]`:
  - `name = "canary-deployer"`
  - `version = "0.0.0"`
  - `requires-python = ">=3.12"`
  - `dependencies = ["trio==0.31.0", "pydantic==2.12.5", "pydantic-settings==2.11.0"]`
- `[build-system]`:
  - `requires = ["hatchling"]`
  - `build-backend = "hatchling.build"`
- `[tool.hatch.build.targets.wheel]`:
  - `include = ["deployer.py"]`

### 2. `canary-deployer/uv.lock` (new, generated)

Run `uv lock` from the `canary-deployer/` directory to generate. Commit the result.

### 3. `canary-deployer/requirements.txt` — delete

### 4. `canary-deployer/__pycache__/` — untrack

Use `git rm -r --cached canary-deployer/__pycache__/`. Already covered by `.gitignore` — no `.gitignore` edit required.

### 5. `mise.toml` — add to `[tools]`

```
python = "3.12"
uv = "latest"
```

### 6. `Dockerfile.canary-deployer` — rework

The `python3` package on `ghcr.io/nais/deploy/deploy:latest` is Python 3.9, which does not satisfy `requires-python = ">=3.12"`. Use uv-managed Python instead — uv will download and provision a 3.12 toolchain automatically based on `requires-python`.

Replace current contents with logic equivalent to:

- `FROM ghcr.io/astral-sh/uv:latest AS uv-provider`
- `FROM ghcr.io/nais/deploy/deploy:latest`
- `COPY --from=uv-provider /uv /usr/local/bin/`
- `WORKDIR /canary/`
- `COPY canary-deployer/pyproject.toml canary-deployer/uv.lock canary-deployer/deployer.py /canary/`
- `RUN uv sync --frozen --no-dev` (uv auto-downloads Python 3.12; creates `/canary/.venv/`)
- `COPY canary-deployer/*.yaml /canary/`
- `RUN /canary/.venv/bin/python -c "import deployer"` (smoke test using venv python)
- `CMD ["/canary/.venv/bin/python", "/canary/deployer.py"]`

Do **not** `apk add python3`. Do not lower the `requires-python` floor.

Note: `uv sync` requires the project files at the working directory; copying `deployer.py` before sync is needed because hatchling will reference it via the wheel `include` config when uv builds/installs the project itself.

## Untouched
- `canary-deployer/deployer.py` — no source changes.
- `canary-deployer/canary.yaml`, `canary-deployer/topic.yaml`.
- `Tiltfile`, `docker-bake.hcl`, `.github/workflows/`, `charts/`, all Go code.
- `.gitignore` (already covers `__pycache__`).

## Non-goals / Later
- Do NOT add dev dependencies (ruff, pytest, etc.).
- Do NOT add a `[project.scripts]` entry; keep invocation as `python deployer.py`.
- Do NOT restructure into a `src/` layout or rename the module.
- Do NOT change the deployer's runtime behavior or dependency versions.
- Do NOT modify Tiltfile, bake file, or workflow — they consume the Dockerfile unchanged.
- Do NOT add a `canary-deployer/.python-version` file (handled by mise).

## Constraints / Caveats
- Dependency versions must be pinned exactly as today.
- `uv.lock` must be generated and committed in this task.
- Use `hatchling` as the build backend.
- The Dockerfile's runtime behavior must match the current image: `python deployer.py` runs with the same dependencies installed; `import deployer` smoke test still passes.
- Do not `apk add python3`; rely on uv-managed Python (`requires-python = ">=3.12"` drives the version).
