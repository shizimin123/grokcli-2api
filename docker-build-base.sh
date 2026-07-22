#!/usr/bin/env bash
# Build the slow runtime dependency layer only when its inputs change.
set -euo pipefail
cd "$(dirname "$0")"

BASE_IMAGE="${GROK2API_BASE_IMAGE:-grokcli-2api-base:local}"
BASE_FINGERPRINT="$({
  sed -n '/^FROM python:3.12-slim-bookworm AS runtime-base$/,/^FROM golang:1.24-bookworm AS go-builder$/p' Dockerfile
  sha256sum requirements.txt requirements-store.txt turnstile-solver/requirements.txt
} | sha256sum | cut -c1-16)"

CURRENT_FINGERPRINT="$(
  docker image inspect \
    --format '{{ index .Config.Labels "com.grokcli-2api.base.fingerprint" }}' \
    "$BASE_IMAGE" 2>/dev/null || true
)"

if [[ "${REBUILD_BASE:-0}" != "1" && "$CURRENT_FINGERPRINT" == "$BASE_FINGERPRINT" ]]; then
  echo "base image current: ${BASE_IMAGE} (${BASE_FINGERPRINT})" >&2
  echo "$BASE_IMAGE"
  exit 0
fi

echo "building base image: ${BASE_IMAGE} (${BASE_FINGERPRINT})" >&2
build_args=(
  --file Dockerfile
  --target runtime-base
  --tag "$BASE_IMAGE"
  --build-arg "GROK2API_BASE_FINGERPRINT=${BASE_FINGERPRINT}"
)
if [[ -n "${BUILD_HTTP_PROXY:-}" ]]; then
  build_args+=(--build-arg "BUILD_HTTP_PROXY=${BUILD_HTTP_PROXY}")
fi
if [[ -n "${BUILD_HTTPS_PROXY:-}" ]]; then
  build_args+=(--build-arg "BUILD_HTTPS_PROXY=${BUILD_HTTPS_PROXY}")
fi
if [[ "${PULL_BASE:-0}" == "1" ]]; then
  build_args+=(--pull)
fi

DOCKER_BUILDKIT=1 docker build "${build_args[@]}" . >&2
echo "$BASE_IMAGE"
