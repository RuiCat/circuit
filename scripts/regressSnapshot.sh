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
# 环境变量：BENCH_CLI=<cli 路径>（显式指定则不重建，仅用于 A/B 对照）、
#           REGRESS_NO_BUILD=1（复用已有二进制，结论自负）、REGRESS_TIMEOUT=120（秒）
# 默认行为：每次调用**重建** $ROOT/.cache/regress-cli，并在输出目录写 _binary.txt 记录其 sha256/HEAD。
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

export GOCACHE="${GOCACHE:-$ROOT/.cache/gocache}"
export GOPATH="${GOPATH:-$ROOT/.cache/gopath}"
export GOMODCACHE="${GOMODCACHE:-$ROOT/.cache/go-mod}"

CLI_BIN="${BENCH_CLI:-$ROOT/.cache/regress-cli}"
TIMEOUT_SEC="${REGRESS_TIMEOUT:-120}"
# 二进制来源：默认**每次重建**（快照的价值全在"这份产物出自哪份代码"，复用旧二进制会让
# "逐位一致"结论变成同一份旧二进制自比——本项目曾因此得出无效的 bitExact 证据）。
# BENCH_CLI 显式指定 ⇒ 调用方负责其新鲜度（A/B 对照用），此处只提示。
# REGRESS_NO_BUILD=1 ⇒ 复用已有二进制（仅限明确知道自己在做什么的实验）。
CLI_FRESHNESS=

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

if [ -n "${BENCH_CLI:-}" ]; then
    if [ ! -x "$CLI_BIN" ]; then
        echo "BENCH_CLI 指定的二进制不存在或不可执行: $CLI_BIN" >&2
        exit 2
    fi
    CLI_FRESHNESS="caller-supplied(BENCH_CLI)"
    echo "[regressSnapshot] 注意: 使用 BENCH_CLI=$CLI_BIN（不自动重建）——请自行确认它包含被测改动" >&2
elif [ "${REGRESS_NO_BUILD:-0}" = "1" ] && [ -x "$CLI_BIN" ]; then
    CLI_FRESHNESS="reused(REGRESS_NO_BUILD=1)"
    echo "[regressSnapshot] 注意: REGRESS_NO_BUILD=1，复用 $CLI_BIN（可能陈旧，结论自负）" >&2
else
    tmpbin="$CLI_BIN.tmp.$$"
    if ! (cd "$ROOT" && go build -o "$tmpbin" ./cmd); then
        rm -f "$tmpbin"
        echo "构建 CLI 失败: $tmpbin" >&2
        exit 2
    fi
    mv -f "$tmpbin" "$CLI_BIN"
    CLI_FRESHNESS="freshly-built"
fi
mkdir -p "$OUTDIR" || exit 2

# 记录产出这份快照的二进制身份：没有它，"逐位一致/有差异"都无法归因到某份代码。
BIN_SHA="$( { sha256sum "$CLI_BIN" 2>/dev/null || shasum -a 256 "$CLI_BIN"; } | awk '{print $1}')"
{
    echo "binary=$CLI_BIN"
    echo "binary_sha256=$BIN_SHA"
    echo "binary_freshness=$CLI_FRESHNESS"
    echo "head_commit=$(cd "$ROOT" && git rev-parse HEAD 2>/dev/null || echo unknown)"
    echo "worktree_dirty_files=$(cd "$ROOT" && git status --porcelain 2>/dev/null | wc -l | tr -d ' ')"
    echo "setfile=$SETFILE"
    echo "generated_at=$(date -Iseconds 2>/dev/null || date)"
} >"$OUTDIR/_binary.txt"

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
