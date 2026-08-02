#!/usr/bin/env bash
#
# 下载前端第三方静态资源到当前目录（ui/vendor/）。
# 文件本身提交进 git，此脚本只是「如何复现」的说明：升级版本时跑一遍覆盖更新。
#
# 用法：
#   ./ui/vendor/fetch.sh
#
set -euo pipefail

# 资源清单：URL | 本地文件名 | 说明
RESOURCES=(
  "https://unpkg.com/alpinejs@3.14.1/dist/cdn.min.js|alpine.min.js|Alpine.js v3.14.1（gzip ~15KB）"
)

cd "$(dirname "$0")"
echo "下载到 $(pwd)/"
for item in "${RESOURCES[@]}"; do
  url="${item%%|*}"
  rest="${item#*|}"
  name="${rest%%|*}"
  desc="${rest#*|}"
  echo "  $name  ←  $desc"
  curl -fsSL -o "$name" "$url"
done
echo "完成。"
