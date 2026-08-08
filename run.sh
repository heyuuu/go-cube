#!/bin/bash
set -euo pipefail

stage() {
  local name=$1
  shift
  echo "==> $name"
  TIMEFORMAT="==> ${name} done (%3Rs)"
  time "$@"
}

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
TMP_BIN="$ROOT_DIR"/tmp/cube

cd "$ROOT_DIR"/server
stage "goimports" goimports -w .
stage "go vet" go vet ./...
stage "go build" go build -o "$TMP_BIN"
"$TMP_BIN" "$@"
