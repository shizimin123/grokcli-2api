# AGENTS.md — grokcli-2api

Working notes for agents and humans operating this repo.

## Deploy branch policy

- **Canonical deploy branch:** `main-rel`
- All production deploys must push/pull/rebuild from `main-rel` (not `main`).
- **Docker build must stay identical to the historical production path:**
  - same files: `Dockerfile`, `docker-build-base.sh`, `docker-rebuild.sh`, `docker-compose.yml`, `entrypoint.sh`
  - same local tags: `grokcli-2api-base:local`, `grokcli-2api:local`
  - same flow: base fingerprint/reuse → app image build → recreate only `grokcli-2api` → wait `/health`
- Do not invent a parallel build pipeline for `main-rel`. Keep docker build scripts unified with production.


## Remote production deploy

Canonical production host:

- SSH: `root@129.211.9.217`
- App dir: `/data/grokcli-2api`
- Compose services: `grokcli-2api`, `redis`, `postgres`
- Published health/API ports: `40081` (and host `3000` mapped to container `40081`)
- Health URL: `http://127.0.0.1:40081/health`

Do **not** hand-edit production app code as the primary deploy path. Always:

1. Commit and push from the dev machine
2. Pull that commit on the server
3. Rebuild/restart with `./docker-rebuild.sh`

### 1. Local: commit + push

```bash
# from repo root on the dev machine
git status -sb
git add <files>
git commit -m "short imperative summary"
git push origin main-rel
```

Confirm the remote tip:

```bash
git log -1 --oneline
git ls-remote origin refs/heads/main-rel
```

### 2. Server: pull cleanly

```bash
ssh root@129.211.9.217
cd /data/grokcli-2api

# always keep secrets
cp -a .env "/tmp/grokcli-2api.env.bak.$(date +%F-%H%M%S)"

git fetch --prune origin
# prefer a clean tree aligned to remote main
git reset --hard origin/main-rel

# keep runtime data + env; remove only blocking untracked deploy junk if needed
# git clean -fd -e data -e .env

git log -1 --oneline   # must match the pushed commit
```

If `git pull --ff-only` fails because the server tree is dirty:

- Prefer `git stash save -u "pre-deploy-..."` when the old git supports it
- If stash is unavailable/fails, backup then `git reset --hard origin/main-rel`
- Never overwrite `/data/grokcli-2api/.env` or `data/`

Note: production remote may be HTTPS (`https://github.com/shizimin123/grokcli-2api.git`)
while the dev machine uses SSH. Both track the same deploy branch (`main-rel`).

### 3. Server: rebuild app container

```bash
cd /data/grokcli-2api
./docker-rebuild.sh
```

What this script does:

- Uses existing `.env` (does not wipe secrets)
- Rebuilds/reuses the runtime base image via `docker-build-base.sh`
- Builds a new app image while the old container keeps serving
- Recreates only the `grokcli-2api` service (redis/postgres stay up)
- Waits until `/health` is ok

Optional private host glue after rebuild (if present, gitignored):

```bash
./docker-rebuild.local.sh
```

### 4. Verify

```bash
docker ps --filter name=grokcli-2api --format '{{.Names}} {{.Status}}'
curl -sS -m 8 http://127.0.0.1:40081/health
# expect: {"status":"ok", ... "ready":true ...}

# confirm expected commit is running from host checkout
cd /data/grokcli-2api && git log -1 --oneline

# confirm Python hot paths are inside the container image
docker exec grokcli-2api python3 -c 'from grok2api.upstream import moemail as m; import inspect; print(inspect.signature(m.cfmail_list_domains))'
```

## Important deploy constraints

- **Python code** (`grok2api/`, `scripts/`, etc.) is copied into the image during rebuild. Host edits alone do **not** affect the running container until rebuild.
- **Go binary**: `docker-rebuild.sh` tries to compile `./bin/grok2api` on the host. If the host has no Go toolchain, existing `bin/grok2api` is kept. For pure-Python changes this is usually fine.
- **Postgres/Redis data** live in compose volumes; do not remove volumes during normal deploys.
- **`.env` is local-only** on the server; it is not sourced from git.

## Docker build (brief)

Multi-stage in `Dockerfile`:

| Stage | Base / tag | Role |
|-------|------------|------|
| `runtime-base` | `python:3.12-slim-bookworm` | OS deps, pip, browsers, docker CLI |
| local cache | `grokcli-2api-base:local` | Fingerprinted base from `./docker-build-base.sh` |
| `go-builder` | `golang:1.24-bookworm` | Build `grok2api` binaries |
| `runtime` | `FROM ${GROK2API_BASE_IMAGE}` | App image → tag `grokcli-2api:local` |

Compose services: app `grokcli-2api:local`, `redis:7-alpine`, `postgres:16-alpine`.

`./docker-rebuild.sh` flow: reuse/build base → build app image → recreate only app container → wait `/health`.
Default `GROK2API_BASE_IMAGE=grokcli-2api-base:local` (compose); standalone default stage is `runtime-base`.

## Registration / CF Temp Email notes

- Protocol registration proxy is stored in DB key `registration_config` (`app_settings`).
- CF Temp Email Worker API calls (`*.workers.dev`) use `_httpx_client` and honor registration `proxy` / `proxy_username` / `proxy_password` when set.
- Env fallback proxies: `GROK2API_XAI_PROXY`, `GROK2API_XAI_PROXY_POOL`, `GROK2API_XAI_PROXY_USERNAME`, `GROK2API_XAI_PROXY_PASSWORD`.
- “测试代理” only smoke-tests reachability of `accounts.x.ai`; it does not by itself prove CF mail API works.
- Prefer filling **CF 域名** explicitly. Empty domain auto-picks from `GET /open_api/settings`.

### Quick CF mail smoke test (on server)

```bash
# uses credentials from registration_config in postgres
docker exec grokcli-2api-postgres psql -U grok2api -d grok2api -t -A \
  -c "select value::text from app_settings where key='registration_config';"
# then run a short python probe inside the app container against
# cfmail_list_domains / cfmail_create_mailbox with proxy=...
```

## Standard commit / deploy checklist

1. Change code locally
2. `git commit` + `git push origin main-rel`
3. Server: `git fetch` + `git reset --hard origin/main-rel`
4. Server: `./docker-rebuild.sh`
5. `curl /health` + targeted smoke test for the change
