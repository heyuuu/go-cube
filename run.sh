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
INVOKED_PWD="$PWD"

# 构建/检查必须在 server/ 下跑（go.mod 在那）；运行回到调用方目录，
# 让 cube 以相对路径解析的参数（如 openapi -o openapi.json）落在用户所在目录
cd "$ROOT_DIR"/server
stage "goimports" goimports -w .
stage "go vet" go vet ./...
stage "go build" go build -o "$TMP_BIN"
cd "$INVOKED_PWD"
"$TMP_BIN" "$@"
