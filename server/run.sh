#!/bin/bash
set -euo pipefail

stage() {
  local name=$1
  shift
  echo "==> $name"
  TIMEFORMAT="==> ${name} done (%3Rs)"
  time "$@"
}

time {
  stage "goimports" goimports -w .
  stage "go build" go build -o tmp/cube
  TIMEFORMAT='==> prepare done (%3Rs), starting ./tmp/cube'
}

./tmp/cube "$@"
