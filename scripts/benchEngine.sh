#!/usr/bin/env bash
# benchEngine.sh：V1 引擎基准——KLU 微基准 + 1e4 RC + 2.3e4 门级 CPU。
# 空匹配 / 无输出一律判 FAIL（不得以 exit 0 假绿）。
set -euo pipefail
cd "$(dirname "$0")/.."
export GOCACHE="${GOCACHE:-$PWD/.cache/go-build}"
export GOPATH="${GOPATH:-$PWD/.cache/gopath}"
export GOMODCACHE="${GOMODCACHE:-$PWD/.cache/go-mod}"

count="${BENCH_COUNT:-1}"

echo "=== KLU（1e4 Poisson / 1e4 MNA / SolveReuse）==="
klu_out="$(go test -run '^$' -bench 'BenchmarkKLU' -benchtime=1x -count="$count" ./maths/ 2>&1)"
echo "$klu_out"
grep -q '^BenchmarkKLU' <<<"$klu_out" || { echo "FAIL: KLU benchmark 未匹配（空匹配）" >&2; exit 1; }

echo "=== 1e4 RC（TestRCScale1e4）==="
rc_out="$(go test -run 'TestRCScale1e4' -v -count=1 -timeout 10m ./cmd/ 2>&1)"
grep -q '元件=' <<<"$rc_out" || { echo "FAIL: TestRCScale1e4 无 元件= 输出" >&2; echo "$rc_out" >&2; exit 1; }
grep '元件=' <<<"$rc_out"

echo "=== 2.3e4 门级 CPU（TestCPU8Scale5x）==="
cpu_out="$(CIRCUIT_MAX_NODES=30000 go test -run 'TestCPU8Scale5x' -v -count=1 -timeout 15m ./cmd/ 2>&1)"
grep -q '元件=' <<<"$cpu_out" || { echo "FAIL: TestCPU8Scale5x 无 元件= 输出" >&2; echo "$cpu_out" >&2; exit 1; }
grep '元件=' <<<"$cpu_out"

echo "=== OK：KLU/RC/CPU8 三组数字均已产出 ==="
