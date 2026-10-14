#!/usr/bin/env bash
# regressSnapshot.sh —— K2：按语料集合跑 CLI 并落逐位可比的 CSV 快照。
#
# 用法：
#   bash scripts/regressSnapshot.sh <语料集合文件> <输出目录>
#
# 输入集合文件：每行 <网表路径>\t<逐字运行参数>（与 runs/circuit-perf-optimization/baselines/corpus.txt 同格式；
#   `#` 开头为注释）。参数按空格拆分后逐字传给 CLI，未给出的参数取 CLI 默认。
# 输出：<输出目录>/<case>.csv（`%.17g` 规范化；原子写：临时文件 + mv）。
#   <case> = 网表文件名去掉 .net；用例名冲突 → 用法错误。
#
# 退出码：0=全部成功 / 1=有用例失败（原始输出保留为 <case>.raw + <case>.err） / 2=用法或输入缺失
#
# 环境变量：BENCH_CLI=<cli 路径>（默认 $ROOT/.cache/m1-cli，不存在则构建）、REGRESS_TIMEOUT=120（秒）
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

export GOCACHE="${GOCACHE:-$ROOT/.cache/gocache}"
export GOPATH="${GOPATH:-$ROOT/.cache/gopath}"
export GOMODCACHE="${GOMODCACHE:-$ROOT/.cache/go-mod}"

CLI_BIN="${BENCH_CLI:-$ROOT/.cache/m1-cli}"
TIMEOUT_SEC="${REGRESS_TIMEOUT:-120}"

if [ $# -ne 2 ]; then
    echo "用法: bash scripts/regressSnapshot.sh <语料集合文件> <输出目录>" >&2
    exit 2
fi
SETFILE="$1"
OUTDIR="$2"
if [ ! -f "$SETFILE" ]; then
    echo "输入缺失: 语料集合文件不存在: $SETFILE" >&2
    exit 2
fi

if [ ! -x "$CLI_BIN" ]; then
    if ! (cd "$ROOT" && go build -o "$CLI_BIN" ./cmd); then
        echo "构建 CLI 失败: $CLI_BIN" >&2
        exit 2
    fi
fi
mkdir -p "$OUTDIR" || exit 2

# 先检查用例名唯一与语料存在（避免跑到一半才发现集合非法）。
names=""
while IFS=$'\t' read -r net args; do
    [ -z "${net:-}" ] && continue
    case "$net" in \#*) continue ;; esac
    if [ ! -f "$ROOT/$net" ]; then
        echo "输入缺失: 语料不存在: $net" >&2
        exit 2
    fi
    name="$(basename "$net" .net)"
    if printf '%s\n' "$names" | grep -qx -- "$name"; then
        echo "用法错误: 用例名重复: $name（不同目录的同名网表）" >&2
        exit 2
    fi
    names="$names$name"$'\n'
done <"$SETFILE"

# 规范化浮点到 %.17g（K2：逐位可比），保留表头行逐字。
normalizeCSV() {
    python3 - "$1" "$2" <<'PY'
import sys
src, dst = sys.argv[1], sys.argv[2]
with open(src, encoding="utf-8") as fh:
    lines = fh.read().splitlines()
if not lines:
    print("快照为空（CLI 无输出）: " + src, file=sys.stderr)
    sys.exit(1)
out = [lines[0]]
for lineno, line in enumerate(lines[1:], start=2):
    cells = []
    for cell in line.split(","):
        try:
            cells.append("%.17g" % float(cell))
        except ValueError:
            print("第 %d 行有非数值单元格 %r: %s" % (lineno, cell, src), file=sys.stderr)
            sys.exit(1)
    out.append(",".join(cells))
with open(dst, "w", encoding="utf-8") as fh:
    fh.write("\n".join(out) + "\n")
PY
}

fail=0
while IFS=$'\t' read -r net args; do
    [ -z "${net:-}" ] && continue
    case "$net" in \#*) continue ;; esac
    name="$(basename "$net" .net)"
    dest="$OUTDIR/$name.csv"
    raw="$OUTDIR/.$name.raw.tmp"
    tmp="$OUTDIR/.$name.csv.tmp"
    err="$OUTDIR/.$name.err.tmp"
    # shellcheck disable=SC2086
    (cd "$ROOT" && timeout "$TIMEOUT_SEC" "$CLI_BIN" -quiet -format csv $args "$net") >"$raw" 2>"$err"
    rc=$?
    if [ "$rc" -ne 0 ]; then
        echo "FAIL: 用例 $name 运行失败（rc=$rc）: $net	$args" >&2
        sed -n '1,3p' "$err" | sed 's/^/       /' >&2
        mv "$raw" "$OUTDIR/$name.raw"
        mv "$err" "$OUTDIR/$name.err"
        fail=1
        continue
    fi
    if ! normalizeCSV "$raw" "$tmp"; then
        echo "FAIL: 用例 $name 的 CSV 规范化失败（原始输出保留）: $net" >&2
        mv "$raw" "$OUTDIR/$name.raw"
        mv "$err" "$OUTDIR/$name.err"
        fail=1
        continue
    fi
    mv "$tmp" "$dest"
    rm -f "$raw" "$err"
    echo "[regressSnapshot] $name → $dest"
done <"$SETFILE"

if [ "$fail" -ne 0 ]; then
    echo "[regressSnapshot] FAIL: 有用例未产出快照" >&2
    exit 1
fi
echo "[regressSnapshot] PASS: 全部用例已写入 $OUTDIR"
exit 0
