#!/usr/bin/env bash
# benchAccuracy/compare.sh：V1 外部精度对比（circuit vs ngspice，DC 工作点）。
# 用法: bash scripts/benchAccuracy/compare.sh
# 输出: 每用例 JSON 行；任一 |rel|>1e-3 时 exit 1。
set -euo pipefail
cd "$(dirname "$0")/../.."
export GOCACHE="${GOCACHE:-$PWD/.cache/go-build}"
export GOPATH="${GOPATH:-$PWD/.cache/gopath}"
export GOMODCACHE="${GOMODCACHE:-$PWD/.cache/go-mod}"
mkdir -p .local/benchAccuracy runs/engine-production/evidence/acc

CASES="01_divider:cmd/circuits/01_resistor_divider.net:cmd/circuits/acc-spice/01_divider.sp:2
02_rc:cmd/circuits/02_rc_filter.net:cmd/circuits/acc-spice/02_rc.sp:2
07_rlc:cmd/circuits/07_rlc_circuit.net:cmd/circuits/acc-spice/07_rlc.sp:3"

fail=0
while IFS=: read -r name net sp node; do
  [ -n "$name" ] || continue
  go run ./cmd -format csv -t 1e-4 "$net" >".local/benchAccuracy/$name.csv" 2>/dev/null
  third_party/reference/ngspice -b -o ".local/benchAccuracy/$name.log" "$sp" >/dev/null 2>&1 || true
  read -r cv nv rel ok < <(python3 - "$name" "$node" <<'PY'
import sys, re
name, node = sys.argv[1], sys.argv[2]
cv = None
with open(f".local/benchAccuracy/{name}.csv") as f:
    rows = [l.strip() for l in f if l.strip()]
hdr = rows[0].split(",")
last = rows[-1].split(",")
cv = float(last[hdr.index(f"node_{node}")])
nv = None
with open(f".local/benchAccuracy/{name}.log") as f:
    rows = re.findall(r"^\s*\d+\s+[0-9.eE+\-]+\s+([0-9.eE+\-]+)", f.read(), re.M)
if rows:
    nv = float(rows[-1])
if nv is None:
    print(cv, "nan", "nan", "0")
else:
    rel = abs(cv - nv) / abs(nv) if nv != 0 else abs(cv)
    print(cv, nv, f"{rel:.3e}", "1" if rel <= 1e-3 else "0")
PY
)
  printf '{"case":"%s","node":%s,"circuit":%s,"ngspice":%s,"rel":%s,"verdict":"%s"}\n' \
    "$name" "$node" "$cv" "$nv" "$rel" "$([ "$ok" = 1 ] && echo PASS || echo FAIL)"
  [ "$ok" = 1 ] || fail=1
done <<<"$CASES"
[ "$fail" = 0 ] || exit 1
