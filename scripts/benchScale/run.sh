#!/usr/bin/env bash
# benchScale/run.sh：1e5 节点规模用例与单步耗时协议（m4，c5）。
# 用法: bash scripts/benchScale/run.sh [N]   （默认 100000）
# 输出: JSON {"nodes":N,"wall_ms":...,"ratio":...}；ratio<5 时 exit 1；CLI 失败 exit 2。
set -euo pipefail
cd "$(dirname "$0")/../.."
export GOCACHE="${GOCACHE:-$PWD/.cache/go-build}"
export GOPATH="${GOPATH:-$PWD/.cache/gopath}"
export GOMODCACHE="${GOMODCACHE:-$PWD/.cache/go-mod}"

N="${1:-100000}"
mkdir -p .local/benchScale runs/engine-production/evidence
GEN=".local/benchScale/rc-${N}.net"
if [ ! -s "$GEN" ]; then
  {
    echo "V1 [1,-1] [0,0,0,0,1]"
    for i in $(seq 2 "$N"); do
      echo "R$i [$((i - 1)),$i] [1000]"
    done
  } >"$GEN"
fi
export CIRCUIT_MAX_NODES=$((N + 10))

start=$(date +%s%N)
set +e
go run ./cmd -t 1e-4 -max-step 1e-5 "$GEN" >/dev/null 2>".local/benchScale/cli.err"
rc=$?
set -e
wall=$((( $(date +%s%N) - start) / 1000000))
if [ "$rc" -ne 0 ]; then
  tail -1 .local/benchScale/cli.err >/dev/null 2>&1 || true
  printf '{"nodes":%d,"wall_ms":%d,"error":"cli_rc_%d","stderr_log":".local/benchScale/cli.err"}\n' "$N" "$wall" "$rc"
  exit 2
fi

BASE="runs/engine-production/evidence/scale-baseline.json"
if [ -f "$BASE" ]; then
  ratio=$(python3 -c "import json;b=json.load(open('$BASE'));print(b['wall_ms']/$wall if $wall>0 else 0)")
else
  printf '{"nodes":%d,"wall_ms":%d}\n' "$N" "$wall" >"$BASE"
  ratio=1
fi
printf '{"nodes":%d,"wall_ms":%d,"ratio":%.2f}\n' "$N" "$wall" "$ratio"
python3 -c "import sys;sys.exit(0 if float('$ratio')>=5 else 1)"
