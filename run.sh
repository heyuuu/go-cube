#!/bin/bash
set -euo pipefail

stage() {
  local name=$1
  shift
  echo "==> $name"
  TIMEFORMAT="==> ${name} done (%3Rs)"
  time "$@"
}

cd server
stage "goimports" goimports -w .
stage "go vet" go vet ./...
stage "go build" go build -o ../tmp/cube
../tmp/cube "$@"
