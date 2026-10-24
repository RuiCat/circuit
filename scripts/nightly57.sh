#!/usr/bin/env bash
# nightly57.sh：57 门级串口全流程夜间回归（C11 降级：不进短门禁）。
# 57 约 2.4s/步、1500 步预算 → 约 60min；用 CIRCUIT_SERIAL_WALL 放宽测试自身 9 分钟墙钟。
set -euo pipefail
cd "$(dirname "$0")/.."
export GOCACHE="${GOCACHE:-$PWD/.cache/go-build}"
export GOPATH="${GOPATH:-$PWD/.cache/gopath}"
export GOMODCACHE="${GOMODCACHE:-$PWD/.cache/go-mod}"

wall="${CIRCUIT_SERIAL_WALL:-90m}"
echo "=== 57 门级串口夜间回归（wall=$wall，CIRCUIT_SERIAL_NET 默认 57）==="
time CIRCUIT_SERIAL_WALL="$wall" timeout 7200 \
  go test -count=1 -timeout 2h -run '^TestCPU8v2SerialLoad$' -v ./cmd/
