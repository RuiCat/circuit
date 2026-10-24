#!/usr/bin/env bash
# benchAccuracy/freeze.sh：冻结 V1 基准 golden（circuit 输出 CSV + sha256 清单）。
# 用法: bash scripts/benchAccuracy/freeze.sh
# 产物: runs/engine-production/evidence/acc-golden/<net>.csv + SHA256SUMS
set -euo pipefail
cd "$(dirname "$0")/../.."
export GOCACHE="${GOCACHE:-$PWD/.cache/go-build}"
export GOPATH="${GOPATH:-$PWD/.cache/gopath}"
export GOMODCACHE="${GOMODCACHE:-$PWD/.cache/go-mod}"

OUT="runs/engine-production/evidence/acc-golden"
mkdir -p "$OUT"
: >"$OUT/SHA256SUMS"
count=0
while IFS=$'\t' read -r net args; do
  case "$net" in ''|'#'*) continue ;; esac
  [ -f "$net" ] || { echo "缺语料: $net" >&2; exit 1; }
  name="$(basename "$net" .net)"
  go run ./cmd -format csv $args "$net" >"$OUT/$name.csv" 2>/dev/null
  sha256sum "$OUT/$name.csv" >>"$OUT/SHA256SUMS"
  count=$((count + 1))
done <cmd/circuits/acc-corpus.txt
echo "{\"cases\":$count,\"dir\":\"$OUT\"}"
