#!/usr/bin/env bash
# benchCompare.sh —— K1：基准与对比口径（group m1..m6 的判定入口，冻结契约见
#   runs/circuit-perf-optimization/40-plan.md「接口契约 K1」）。
#
# 用法：
#   bash scripts/benchCompare.sh --group <m1..m6|all> [--baseline <file>] [--write-baseline]
#                                [--selfcheck] [--json <out>] [--results-dir <dir>]
#
# 退出码：0=全 PASS（无 FAIL，且至少 1 个条目可判） / 1=有 FAIL / 2=用法或基线缺失
#         3=给不出判定（--write-baseline 时 maxMinRatio>1.15 拒绝写；compare 时 judged==0 即 NOT_JUDGEABLE，D14）
#
# 语义要点（逐字口径见 baselines/thresholds.md）：
#   - 每次运行（含失败）都写 <results>/<group>/{bench.txt,bench.json,verdict.json}；
#   - --write-baseline 先打印实测 maxMinRatio 与 loadAvg，>1.15 时拒绝写入并返回 3（每轮自标定）；
#   - 基线/快照一律原子写（临时文件 + mv）；
#   - 默认不依赖调用方的 Go 缓存环境：未设置时脚本自行指向 $ROOT/.cache/*（C14）。
#
# 环境变量：
#   BENCH_CLI=<path>       自检用的 CLI 二进制（默认 $ROOT/.cache/m1-cli）
#   BENCH_OVERLAY=<json>   仅本地验证用：给 go test 透传 -overlay（会原样记入基线命令头）
#   BENCH_GROUPS_FILE=<tsv> 名单表覆盖（仅用于复现阈值标定门/实验，正常验收不要设）
#   BENCH_BASELINE_DIR=<dir>  基线目录覆盖（默认 runs/.../baselines；沙箱自测用）
#   BENCH_THRESHOLDS_FILE=<md> 阈值文档覆盖（默认 <基线目录>/thresholds.md）
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

export GOCACHE="${GOCACHE:-$ROOT/.cache/gocache}"
export GOPATH="${GOPATH:-$ROOT/.cache/gopath}"
export GOMODCACHE="${GOMODCACHE:-$ROOT/.cache/go-mod}"

GROUPS_FILE="${BENCH_GROUPS_FILE:-$SCRIPT_DIR/benchGroups.tsv}"
SELFCHECK_PY="$SCRIPT_DIR/benchSelfcheck.py"
RUNS_DIR="$ROOT/runs/circuit-perf-optimization"
BASELINE_DIR="${BENCH_BASELINE_DIR:-$RUNS_DIR/baselines}"
HISTORY_DIR="$BASELINE_DIR/history"
THRESHOLDS="${BENCH_THRESHOLDS_FILE:-$BASELINE_DIR/thresholds.md}"
THRESHOLDS_PY="$SCRIPT_DIR/benchThresholds.py"
CORPUS="$BASELINE_DIR/corpus.txt"
CORPUS_EXCLUDED="$BASELINE_DIR/corpus-excluded.txt"
RESULTS_ROOT="$RUNS_DIR/_results"
CLI_BIN="${BENCH_CLI:-$ROOT/.cache/m1-cli}"
THRESHOLD_LIMIT=1.15

GROUP=""
BASELINE=""
WRITE_BASELINE=0
SELFCHECK=0
JSON_OUT=""
RESULTS_DIR=""

# groupRows <group>：打印该 group 的名单行（TAB 分隔 5 列）。
groupRows() {
    awk -F'\t' -v g="$1" '!/^[[:space:]]*#/ && NF==5 && $1==g' "$GROUPS_FILE"
}

# pkgSlug <pkg>：包路径 → 文件名安全片段。
pkgSlug() {
    printf '%s' "$1" | tr -c 'A-Za-z0-9' '_'
}

# commitId：HEAD 短哈希；有未提交改动则加 +dirty（基线头必须能指认被测状态）。
commitId() {
    local c
    c="$(git -C "$ROOT" rev-parse --short HEAD 2>/dev/null || echo unknown)"
    if ! git -C "$ROOT" diff --quiet 2>/dev/null; then
        c="$c+dirty"
    fi
    printf '%s' "$c"
}

# loadAvg：采集时 load average（基线头必填）。
loadAvg() {
    uptime | sed 's/.*load average:[[:space:]]*//'
}

usage() {
    sed -n '2,26p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

failUsage() {
    echo "[benchCompare] 用法错误: $1" >&2
    echo "[benchCompare] 可用 group: m1 m2 m3 m4 m5 m6 all" >&2
    exit 2
}

# resultsDirFor <group>：结果目录（--results-dir 覆盖，用于基准标定时不写他人写域）。
# 一律返回绝对路径：go test 的工作目录是包目录，相对路径会落到包目录下。
resultsDirFor() {
    local base
    if [ -n "$RESULTS_DIR" ]; then
        case "$RESULTS_DIR" in
            /*) base="$RESULTS_DIR" ;;
            *) base="$ROOT/$RESULTS_DIR" ;;
        esac
    else
        base="$RESULTS_ROOT"
    fi
    printf '%s/%s' "$base" "$1"
}

# absPath <path>：相对路径按仓库根解析为绝对路径（go test 的 CWD 是包目录）。
absPath() {
    case "$1" in
        /*) printf '%s' "$1" ;;
        *) printf '%s/%s' "$ROOT" "$1" ;;
    esac
}

# writeFailureVerdict <dir> <group> <baseline> <reason>：运行失败也要落 verdict.json。
writeFailureVerdict() {
    local dir="$1" group="$2" baseline="$3" reason="$4"
    mkdir -p "$dir"
    python3 - "$dir/verdict.json" "$group" "$baseline" "$reason" <<'PY'
import json, sys
out, group, baseline, reason = sys.argv[1:5]
doc = {
    "group": group, "commit": "", "verdict": "FAIL", "baselineFile": baseline,
    "currentFile": "", "judgedCount": 0, "perBenchmark": [], "ungaugeable": [],
    "reason": reason,
}
with open(out + ".tmp", "w", encoding="utf-8") as fh:
    json.dump(doc, fh, ensure_ascii=False, indent=2)
    fh.write("\n")
import os
os.replace(out + ".tmp", out)
PY
}

# runBenchRows <group> <dir>：逐行跑 go test -bench，聚合出 bench.json/bench.csv。
runBenchRows() {
    local group="$1" dir="$2"
    mkdir -p "$dir"
    : > "$dir/bench.txt"
    local -a raws=() cmds=() expects=()
    local benchtime="" count="" row pkg regex bt cnt rc slug raw cmdstr
    while IFS=$'\t' read -r row pkg regex bt cnt; do
        [ -z "${row:-}" ] && continue
        if [ -z "$benchtime" ]; then
            benchtime="$bt"
            count="$cnt"
        fi
        if [ "$bt" != "$benchtime" ] || [ "$cnt" != "$count" ]; then
            echo "[benchCompare] FAIL: group $group 的 benchtime/count 在名单内不一致" >&2
            return 1
        fi
        slug="$(pkgSlug "$pkg")"
        raw="$dir/raw-$slug.txt"
        local -a cmd=(go test)
        if [ -n "${BENCH_OVERLAY:-}" ]; then
            cmd+=(-overlay "$BENCH_OVERLAY")
        fi
        cmd+=(-count="$cnt" -run '^$' -bench "$regex" -benchtime="$bt" -benchmem "$pkg")
        cmdstr="$(printf '%q ' "${cmd[@]}")"
        {
            printf '# group=%s pkg=%s benchtime=%s count=%s\n' "$group" "$pkg" "$bt" "$cnt"
            printf '# cmd: %s\n' "$cmdstr"
        } >> "$dir/bench.txt"
        echo "[benchCompare] $cmdstr"
        (cd "$ROOT" && "${cmd[@]}") >"$raw" 2>&1
        rc=$?
        cat "$raw" >>"$dir/bench.txt"
        if [ "$rc" -ne 0 ]; then
            echo "[benchCompare] FAIL(rc=$rc): $cmdstr" >&2
            sed -n '1,20p' "$raw" | sed 's/^/[benchCompare]   /' >&2
            return 1
        fi
        raws+=("$pkg=$raw")
        cmds+=("$cmdstr")
        expects+=("$regex")
    done < <(groupRows "$group")
    if [ "${#raws[@]}" -eq 0 ]; then
        echo "[benchCompare] FAIL: group $group 未在 benchGroups.tsv 中定义" >&2
        return 1
    fi
    local expect_union
    expect_union="$(printf '%s|' "${expects[@]}")"
    expect_union="${expect_union%|}"
    local -a agg=(go test -count=1 -run '^TestBenchlibAggregate$' ./maths/ -args)
    agg+=(-benchlib-group "$group" -benchlib-commit "$(commitId)" -benchlib-loadavg "$(loadAvg)")
    agg+=(-benchlib-benchtime "$benchtime" -benchlib-count "$count")
    local x
    for x in "${raws[@]}"; do agg+=(-benchlib-raw "$x"); done
    for x in "${cmds[@]}"; do agg+=(-benchlib-command "$x"); done
    agg+=(-benchlib-json "$dir/bench.json" -benchlib-csv "$dir/bench.csv" -benchlib-expect "$expect_union")
    (cd "$ROOT" && "${agg[@]}") >>"$dir/bench.txt" 2>&1
    rc=$?
    if [ "$rc" -ne 0 ] || [ ! -s "$dir/bench.json" ]; then
        echo "[benchCompare] FAIL(rc=$rc): 聚合 bench.json 失败（见 $dir/bench.txt）" >&2
        return 1
    fi
    return 0
}

# writeBaselineForGroup <group> <dir>：标定并原子写基线；>1.15 → 3。
# 写前把旧基线备份到 history/<group>-<旧 generatedAt>.json（原子写），新基线里记 previousBaseline，
# 并把「旧值→新值」累计到 $CALIB_CHANGES（由主循环统一追加到 thresholds.md 的变更记录）。
writeBaselineForGroup() {
    local group="$1" dir="$2"
    if ! runBenchRows "$group" "$dir"; then
        writeFailureVerdict "$dir" "$group" "$BASELINE_DIR/$group.json" "benchmark 运行失败"
        return 1
    fi
    local ratio
    ratio="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1],encoding="utf-8"))["thresholdMaxMinRatio"])' "$dir/bench.json")"
    echo "[benchCompare] group=$group 标定: thresholdMaxMinRatio=$ratio loadAvg=$(loadAvg) benchtime/命令见 $dir/bench.txt"
    writeFailureVerdict "$dir" "$group" "$BASELINE_DIR/$group.json" "本次为标定，不做判定（thresholdMaxMinRatio=$ratio）"
    python3 - "$dir/verdict.json" "$ratio" <<'PYVERDICT'
import json, sys, os
path, ratio = sys.argv[1], float(sys.argv[2])
with open(path, encoding="utf-8") as fh:
    doc = json.load(fh)
doc["verdict"] = "CALIBRATED"
doc["thresholdMaxMinRatio"] = ratio
with open(path + ".tmp", "w", encoding="utf-8") as fh:
    json.dump(doc, fh, ensure_ascii=False, indent=2)
    fh.write("\n")
os.replace(path + ".tmp", path)
PYVERDICT
    if python3 -c 'import sys;raise SystemExit(0 if float(sys.argv[1])>float(sys.argv[2]) else 1)' "$ratio" "$THRESHOLD_LIMIT"; then
        echo "[benchCompare] 拒绝写基线: maxMinRatio=$ratio > $THRESHOLD_LIMIT（K1：阈值标定不可用，原始输出保留在 $dir/）" >&2
        return 3
    fi
    mkdir -p "$BASELINE_DIR" "$HISTORY_DIR"
    local meta
    meta="$(python3 - "$BASELINE_DIR/$group.json" "$dir/bench.json" "$HISTORY_DIR" "$group" <<'PYMETA'
import datetime, json, os, sys

old_path, new_path, history_dir, group = sys.argv[1:5]
old_ratio = ""
history_rel = ""
if os.path.isfile(old_path):
    with open(old_path, encoding="utf-8") as fh:
        old = json.load(fh)
    old_ratio = "%.9f" % old.get("thresholdMaxMinRatio", 0)
    stamp = (old.get("generatedAt") or "").replace("+08:00", "")
    if len(stamp) >= 16:
        stamp = stamp[:16].replace(":", "")          # 2026-10-12T2350
    else:
        stamp = datetime.datetime.fromtimestamp(os.path.getmtime(old_path)).strftime("%Y-%m-%dT%H%M")
    base = "%s-%s" % (group, stamp)
    target = os.path.join(history_dir, base + ".json")
    n = 2
    while os.path.exists(target):
        target = os.path.join(history_dir, "%s-%d.json" % (base, n))
        n += 1
    with open(target + ".tmp", "w", encoding="utf-8") as fh:
        json.dump(old, fh, ensure_ascii=False, indent=2)
        fh.write("\n")
    os.replace(target + ".tmp", target)
    history_rel = "history/" + os.path.basename(target)
with open(new_path, encoding="utf-8") as fh:
    fresh = json.load(fh)
fresh["previousBaseline"] = history_rel if history_rel else "（首次写入，无历史）"
with open(old_path + ".tmp", "w", encoding="utf-8") as fh:
    json.dump(fresh, fh, ensure_ascii=False, indent=2)
    fh.write("\n")
os.replace(old_path + ".tmp", old_path)
print("%s\t%.9f\t%s" % (old_ratio if old_ratio else "（无）", fresh.get("thresholdMaxMinRatio", 0), history_rel if history_rel else "（首次写入，无历史）"))
PYMETA
)"
    if [ -n "$CALIB_CHANGES" ]; then
        printf '%s\t%s\t%s\t%s\t%s\n' "$group" \
            "$(printf '%s' "$meta" | cut -f1)" "$(printf '%s' "$meta" | cut -f2)" \
            "$(printf '%s' "$meta" | cut -f3)" "$(loadAvg)" >>"$CALIB_CHANGES"
    fi
    echo "[benchCompare] 已写基线 $BASELINE_DIR/$group.json（历史: $(printf '%s' "$meta" | cut -f3)）"
    return 0
}

# compareGroup <group> <dir>：跑基准 + 与基线对比 → verdict.json。
compareGroup() {
    local group="$1" dir="$2"
    local base="${BASELINE:-$BASELINE_DIR/$group.json}"
    if ! runBenchRows "$group" "$dir"; then
        writeFailureVerdict "$dir" "$group" "$base" "benchmark 运行失败"
        return 1
    fi
    if [ ! -f "$base" ]; then
        writeFailureVerdict "$dir" "$group" "$base" "基线缺失：禁止用当前值当基线"
        echo "[benchCompare] 基线缺失: $base（先 --write-baseline）" >&2
        return 2
    fi
    local -a verdict=(go test -count=1 -run '^TestBenchlibVerdict$' ./maths/ -args
        -benchlib-baseline "$base" -benchlib-current "$dir/bench.json" -benchlib-verdict "$dir/verdict.json")
    (cd "$ROOT" && "${verdict[@]}") >>"$dir/bench.txt" 2>&1
    local rc=$?
    if [ ! -s "$dir/verdict.json" ]; then
        writeFailureVerdict "$dir" "$group" "$base" "判定失败（见 bench.txt）"
        return 1
    fi
    if [ -n "$JSON_OUT" ]; then
        cp "$dir/verdict.json" "$JSON_OUT.tmp.$$" && mv "$JSON_OUT.tmp.$$" "$JSON_OUT"
    fi
    # D14：judged==0（全部 entry 落在噪声地板内）→ 给不出判定，退出码 3，禁止读成 PASS。
    local label
    label="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1],encoding="utf-8")).get("verdict",""))' "$dir/verdict.json")"
    if [ "$label" = "NOT_JUDGEABLE" ]; then
        echo "[benchCompare] group=$group verdict=NOT_JUDGEABLE（judged=0，全部 entry 落在噪声地板内，D14）→ 退出码 3" >&2
        return 3
    fi
    return "$rc"
}

# runCli <net> [args...]：跑一次 CLI，输出落 $TMP_RUN_OUT/$TMP_RUN_ERR，退出码落 CLI_RC。
runCli() {
    local net="$1"
    shift
    # shellcheck disable=SC2086
    (cd "$ROOT" && timeout 60 "$CLI_BIN" -quiet "$@" "$net") >"$TMP_RUN_OUT" 2>"$TMP_RUN_ERR"
    CLI_RC=$?
}

# csvNormalizable <file>：第 2 行是否可整体解析为 CSV 数值（k2-shape 判据用）。
csvNormalizable() {
    python3 - "$1" <<'PY'
import sys
lines = open(sys.argv[1], encoding="utf-8").read().splitlines()
if len(lines) < 2:
    raise SystemExit(1)
for cell in lines[1].split(","):
    float(cell)
PY
}

# checkCorpusAvailability：语料可用性判据。
#   corpus.txt       逐条要求 rc=0；
#   corpus-excluded  rc=<n> 条目要求实测 rc 与登记一致且非 0；k2-shape 条目要求实测 rc=0 且 CSV 规范化确实失败。
checkCorpusAvailability() {
    local tmpdir crit rc net args reason recorded fail=0
    tmpdir="$(mktemp -d)"
    TMP_RUN_OUT="$tmpdir/out"
    TMP_RUN_ERR="$tmpdir/err"
    if ! (cd "$ROOT" && go build -o "$CLI_BIN" ./cmd) 2>"$tmpdir/build.err"; then
        echo "FAIL: CLI 构建失败（$CLI_BIN）: $(head -c 200 "$tmpdir/build.err")"
        rm -rf "$tmpdir"
        return 1
    fi
    while IFS=$'\t' read -r net args; do
        [ -z "${net:-}" ] && continue
        case "$net" in \#*) continue ;; esac
        # shellcheck disable=SC2086
        runCli "$net" $args
        if [ "$CLI_RC" -ne 0 ]; then
            echo "FAIL: corpus 条目不满足判据（期望 rc=0，实测 rc=$CLI_RC）: $net	$args"
            echo "      stderr: $(head -c 160 "$TMP_RUN_ERR" | tr '\n' ' ')"
            fail=1
        fi
    done <"$CORPUS"
    while IFS=$'\t' read -r net crit reason; do
        [ -z "${net:-}" ] && continue
        case "$net" in \#*) continue ;; esac
        case "$crit" in
            rc=*)
                recorded="${crit#rc=}"
                runCli "$net" -t 1e-4
                if [ "$CLI_RC" -eq 0 ]; then
                    echo "FAIL: corpus-excluded(rc) 条目实测可用（rc=0），应改入 corpus.txt: $net"
                    fail=1
                elif [ "$CLI_RC" -ne "$recorded" ]; then
                    echo "FAIL: corpus-excluded 条目退出码与登记不符（登记 rc=$recorded，实测 rc=$CLI_RC）: $net"
                    fail=1
                fi
                ;;
            k2-shape)
                runCli "$net" -t 1e-4
                if [ "$CLI_RC" -ne 0 ]; then
                    echo "FAIL: corpus-excluded(k2-shape) 条目要求 rc=0，实测 rc=$CLI_RC: $net"
                    fail=1
                elif csvNormalizable "$TMP_RUN_OUT" 2>/dev/null; then
                    echo "FAIL: corpus-excluded(k2-shape) 条目实测可规范化（应改入 corpus.txt）: $net"
                    fail=1
                fi
                ;;
            *)
                echo "FAIL: corpus-excluded 判据列无法识别（要求 rc=<n> 或 k2-shape）: $net	$crit"
                fail=1
                ;;
        esac
    done <"$CORPUS_EXCLUDED"
    rm -rf "$tmpdir"
    return "$fail"
}

# runSelfcheck：静态自检 + 语料可用性判据。
runSelfcheck() {
    python3 "$SELFCHECK_PY" --root "$ROOT" --groups "$GROUPS_FILE" \
        --baselines "$BASELINE_DIR" --thresholds "$THRESHOLDS" \
        --corpus "$CORPUS" --excluded "$CORPUS_EXCLUDED" --require-baselines || return 1
    checkCorpusAvailability || return 1
    echo "[benchCompare] SELFCHECK PASS: 名单/基线/阈值/语料可用性全部一致"
    return 0
}

while [ $# -gt 0 ]; do
    case "$1" in
        --group) GROUP="${2:-}"; shift 2 ;;
        --baseline) BASELINE="${2:-}"; shift 2 ;;
        --write-baseline) WRITE_BASELINE=1; shift ;;
        --selfcheck) SELFCHECK=1; shift ;;
        --json) JSON_OUT="${2:-}"; shift 2 ;;
        --results-dir) RESULTS_DIR="${2:-}"; shift 2 ;;
        -h|--help) usage; exit 0 ;;
        *) failUsage "未知参数 $1" ;;
    esac
done

[ -n "$GROUP" ] || failUsage "缺少 --group"
case "$GROUP" in
    m1|m2|m3|m4|m5|m6|all) ;;
    *) failUsage "非法 group '$GROUP'" ;;
esac
if [ -n "$BASELINE" ] && [ "$GROUP" = "all" ]; then
    failUsage "--baseline 只能配合单个 group 使用"
fi
if [ -n "$BASELINE" ]; then
    BASELINE="$(absPath "$BASELINE")"
fi
if [ -n "$JSON_OUT" ]; then
    JSON_OUT="$(absPath "$JSON_OUT")"
fi

if [ "$SELFCHECK" -eq 1 ]; then
    runSelfcheck
    exit $?
fi

groupsToRun=()
if [ "$GROUP" = "all" ]; then
    groupsToRun=(m1 m2 m3 m4 m5 m6)
else
    groupsToRun=("$GROUP")
fi

CALIB_CHANGES=""
if [ "$WRITE_BASELINE" -eq 1 ]; then
    CALIB_CHANGES="$(mktemp)"
fi

overall=0
for g in "${groupsToRun[@]}"; do
    dir="$(resultsDirFor "$g")"
    if [ "$WRITE_BASELINE" -eq 1 ]; then
        writeBaselineForGroup "$g" "$dir"
        rc=$?
    else
        compareGroup "$g" "$dir"
        rc=$?
    fi
    echo "[benchCompare] group=$g rc=$rc"
    case "$rc" in
        0) ;;
        3) [ "$overall" -lt 3 ] && overall=3 ;;
        2) [ "$overall" -lt 2 ] && overall=2 ;;
        *) [ "$overall" -eq 0 ] || true; [ "$overall" -lt 1 ] && overall=1 ;;
    esac
done
if [ "$WRITE_BASELINE" -eq 1 ] && [ -n "$CALIB_CHANGES" ]; then
    # 基线是权威数值 → 同步 thresholds.md 数值区；并把「旧值→新值」追加进变更记录（C10）。
    if [ -f "$THRESHOLDS_PY" ] && [ -f "$THRESHOLDS" ]; then
        if ! python3 "$THRESHOLDS_PY" sync --baselines "$BASELINE_DIR" --thresholds "$THRESHOLDS"; then
            echo "[benchCompare] 警告: thresholds.md 数值区同步失败（--selfcheck 会报不一致）" >&2
        fi
        if [ -s "$CALIB_CHANGES" ]; then
            changes_tsv="$CALIB_CHANGES.tsv"
            printf '# group\toldRatio\tnewRatio\thistoryFile\tloadAvg\tcommit\treason\n' >"$changes_tsv"
            awk -F'\t' -v c="$(commitId)" '{printf "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", $1, $2, $3, $4, $5, c, "--write-baseline 重新标定"}' "$CALIB_CHANGES" >>"$changes_tsv"
            if ! python3 "$THRESHOLDS_PY" log-change --thresholds "$THRESHOLDS" --tsv "$changes_tsv"; then
                echo "[benchCompare] 警告: thresholds.md 变更记录追加失败" >&2
            fi
        fi
    else
        echo "[benchCompare] 警告: 缺少 $THRESHOLDS_PY 或 $THRESHOLDS，跳过阈值文档同步" >&2
    fi
    rm -f "$CALIB_CHANGES" "$CALIB_CHANGES.tsv"
fi
if [ -n "$JSON_OUT" ] && [ "$GROUP" = "all" ]; then
    python3 - "$JSON_OUT" "$RESULTS_DIR" "$RESULTS_ROOT" "${groupsToRun[@]}" <<'PY'
import json, os, sys
out, override, root, groups = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4:]
docs = {}
for g in groups:
    path = os.path.join(override or root, g, "verdict.json")
    if os.path.isfile(path):
        with open(path, encoding="utf-8") as fh:
            docs[g] = json.load(fh)
with open(out + ".tmp", "w", encoding="utf-8") as fh:
    json.dump({"verdicts": docs}, fh, ensure_ascii=False, indent=2)
    fh.write("\n")
os.replace(out + ".tmp", out)
PY
fi
exit "$overall"
