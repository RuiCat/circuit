#!/usr/bin/env python3
"""benchSelfcheck.py —— K1 静态自检（不跑基准，只核对口径一致性）。

被 scripts/benchCompare.sh --selfcheck 调用；只用 python3 标准库。

检查项：
  1. scripts/benchGroups.tsv：group 取值、benchtime/count 同 group 一致、正则能匹配到真实 Benchmark；
  2. baselines/<group>.json：必需字段、runs/nsPerOp 长度、thresholdMaxMinRatio=max(entry) 且 ≤1.15、
     条目集合与实际 Benchmark 集合逐名一致（改名/漏跑即报错）；
  3. baselines/thresholds.md：机读块与基线 JSON 的 thresholdMaxMinRatio/perfThresholdPct 一致，
     且「性能阈值」与「噪声地板」两个身份都出现（C10）；
  4. baselines/corpus.txt + corpus-excluded.txt：显式列举、可解析、两者并集覆盖 cmd/circuits/*.net 全部网表。

退出码：0=通过 / 1=有检查项失败 / 2=用法或输入缺失。
"""

import argparse
import glob
import json
import os
import re
import sys

GROUPS = ["m1", "m2", "m3", "m4", "m5", "m6"]
THRESHOLD_MAX_MIN_RATIO = 1.15
# 跨窗口复现性下限（%），与 maths/benchlib_test.go 的 benchlibMinWindowFloorPct 保持一致
MIN_WINDOW_FLOOR_PCT = 5.0
REQUIRED_DOC_FIELDS = [
    "group", "commit", "goVersion", "nproc", "loadAvg", "benchtime", "count", "runs",
    "statistic", "thresholdMaxMinRatio", "perfThresholdPct", "noiseSafetyFactor",
    "entries",
]
REQUIRED_ENTRY_FIELDS = ["name", "package", "nsPerOp", "maxMinRatio", "bytesPerOp", "allocsPerOp"]

errors = []
notes = []


def fail(msg):
    errors.append(msg)


def parseGroups(path):
    """解析 benchGroups.tsv，返回 row dict 列表，并检查 group 取值/正则/benchtime/count。"""
    rows = []
    with open(path, encoding="utf-8") as fh:
        for lineno, line in enumerate(fh, 1):
            line = line.rstrip("\n")
            if not line.strip() or line.lstrip().startswith("#"):
                continue
            parts = line.split("\t")
            if len(parts) != 5:
                fail("benchGroups.tsv:%d 列数 %d != 5: %r" % (lineno, len(parts), line))
                continue
            group, pkg, regex, benchtime, count = parts
            if group not in GROUPS:
                fail("benchGroups.tsv:%d 未知 group %r" % (lineno, group))
            try:
                count = int(count)
            except ValueError:
                fail("benchGroups.tsv:%d count 不是整数: %r" % (lineno, count))
                continue
            try:
                re.compile(regex)
            except re.error as exc:
                fail("benchGroups.tsv:%d 非法正则: %s" % (lineno, exc))
                continue
            if count < 2:
                fail("benchGroups.tsv:%d count=%d <2，无法给出 maxMinRatio" % (lineno, count))
            if not benchtime:
                fail("benchGroups.tsv:%d benchtime 为空" % lineno)
            rows.append(dict(group=group, pkg=pkg, regex=regex, benchtime=benchtime, count=count))
    if not rows:
        fail("benchGroups.tsv 没有任何 group 行")
    for group in GROUPS:
        if not [r for r in rows if r["group"] == group]:
            fail("benchGroups.tsv 缺少 group %s" % group)
    return rows


def packageBenchmarks(root, pkg):
    """扫描包目录下的 *.go，返回全部 `func Benchmark*` 名字集合（非递归，天然不含 .cache/）。"""
    """返回包目录下所有 Benchmark 函数名（非递归；天然不含 .cache/）。"""
    pkgdir = os.path.join(root, pkg.lstrip("./"))
    names = set()
    for path in sorted(glob.glob(os.path.join(pkgdir, "*.go"))):
        with open(path, encoding="utf-8") as fh:
            names.update(re.findall(r"^func\s+(Benchmark\w+)", fh.read(), re.M))
    return names


def checkBaseline(path, group, rows, root):
    """核对单个基线 JSON 的字段、样本数、阈值与"实际 Benchmark 集合"是否逐名一致。"""
    if not os.path.isfile(path):
        fail("基线缺失: %s（--write-baseline 后才能对比）" % path)
        return None
    with open(path, encoding="utf-8") as fh:
        doc = json.load(fh)
    for field in REQUIRED_DOC_FIELDS:
        if field not in doc:
            fail("%s 缺字段 %s" % (path, field))
    if doc.get("group") != group:
        fail("%s group=%r != %s" % (path, doc.get("group"), group))
    for field in ("commit", "goVersion", "loadAvg", "benchtime", "statistic"):
        if not doc.get(field):
            fail("%s 字段 %s 为空（基线头必须逐字记录）" % (path, field))
    if doc.get("nproc", 0) <= 0:
        fail("%s nproc 非法: %r" % (path, doc.get("nproc")))
    ratio = doc.get("thresholdMaxMinRatio", 0)
    if not (0 < ratio <= THRESHOLD_MAX_MIN_RATIO):
        fail("%s thresholdMaxMinRatio=%.4f 不在 (0, %.2f]" % (path, ratio, THRESHOLD_MAX_MIN_RATIO))
    if doc.get("perfThresholdPct", 0) <= 0:
        fail("%s perfThresholdPct 非法: %r" % (path, doc.get("perfThresholdPct")))
    if doc.get("noiseSafetyFactor", 0) <= 0:
        fail("%s noiseSafetyFactor 非法: %r" % (path, doc.get("noiseSafetyFactor")))
    benchtimes = sorted({r["benchtime"] for r in rows})
    counts = sorted({r["count"] for r in rows})
    if len(benchtimes) != 1:
        fail("group %s 的 benchtime 不唯一: %s" % (group, benchtimes))
    if len(counts) != 1:
        fail("group %s 的 count 不唯一: %s" % (group, counts))
    if doc.get("benchtime") != benchtimes[0]:
        fail("%s benchtime=%r != 名单的 %r" % (path, doc.get("benchtime"), benchtimes[0]))
    if doc.get("count") != counts[0] or doc.get("runs") != counts[0]:
        fail("%s count/runs=%r/%r != 名单的 %d" % (path, doc.get("count"), doc.get("runs"), counts[0]))
    entries = doc.get("entries") or []
    if not entries:
        fail("%s entries 为空" % path)
        return doc
    worst = 0.0
    for entry in entries:
        for field in REQUIRED_ENTRY_FIELDS:
            if field not in entry:
                fail("%s entry %r 缺字段 %s" % (path, entry.get("name"), field))
        if len(entry.get("nsPerOp") or []) != doc.get("runs"):
            fail("%s entry %s 的 nsPerOp 样本数 %d != runs %r"
                 % (path, entry.get("name"), len(entry.get("nsPerOp") or []), doc.get("runs")))
        if not (0 < entry.get("maxMinRatio", 0) <= THRESHOLD_MAX_MIN_RATIO):
            fail("%s entry %s maxMinRatio=%r 越界" % (path, entry.get("name"), entry.get("maxMinRatio")))
        worst = max(worst, entry.get("maxMinRatio", 0))
    if abs(worst - ratio) > 1e-9:
        fail("%s thresholdMaxMinRatio=%.6f != max(entry.maxMinRatio)=%.6f" % (path, ratio, worst))
    prev = doc.get("previousBaseline")
    if prev and prev != "（首次写入，无历史）":
        hist_path = os.path.join(os.path.dirname(path), prev)
        if not os.path.isfile(hist_path):
            fail("%s 的 previousBaseline=%r 指向不存在的历史基线（基线历史机制被破坏）" % (path, prev))
    expected = set()
    for row in rows:
        expected.update((row["pkg"], n) for n in packageBenchmarks(root, row["pkg"]) if re.search(row["regex"], n))
    actual = set((e.get("package"), e.get("name")) for e in entries)
    if expected != actual:
        missing = sorted(expected - actual)
        extra = sorted(actual - expected)
        fail("group %s 基线名单与实际 Benchmark 集合不一致: 缺 %s / 多 %s" % (group, missing, extra))
    return doc


def parseThresholdBlock(path):
    """解析 thresholds.md 的机读块（BENCH-THRESHOLDS-BEGIN/END），返回 {group: {key: float}}。"""
    """解析 thresholds.md 的机读块：<!-- BENCH-THRESHOLDS-BEGIN ... BENCH-THRESHOLDS-END -->"""
    with open(path, encoding="utf-8") as fh:
        text = fh.read()
    if "性能阈值" not in text or "噪声地板" not in text:
        fail("%s 必须同时出现「性能阈值」与「噪声地板」（C10 身份分离）" % path)
    match = re.search(r"BENCH-THRESHOLDS-BEGIN(.*?)BENCH-THRESHOLDS-END", text, re.S)
    if not match:
        fail("%s 缺机读块 BENCH-THRESHOLDS-BEGIN/END" % path)
        return {}
    parsed = {}
    for line in match.group(1).splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        parts = line.split()
        group = parts[0]
        values = {}
        for token in parts[1:]:
            if "=" not in token:
                fail("%s 机读块 token 非法: %r" % (path, token))
                continue
            key, value = token.split("=", 1)
            try:
                values[key] = float(value)
            except ValueError:
                fail("%s 机读块 %s 的值不是数字: %r" % (path, key, value))
        parsed[group] = values
    return parsed


def checkThresholdsDoc(path, baselines):
    """核对 thresholds.md 与基线 JSON 的 thresholdMaxMinRatio/perfThresholdPct/noiseFloorPct 一致，并检查两个身份都在场。"""
    if not os.path.isfile(path):
        fail("阈值文档缺失: %s" % path)
        return
    parsed = parseThresholdBlock(path)
    for group, doc in baselines.items():
        if doc is None:
            continue
        values = parsed.get(group)
        if values is None:
            fail("%s 机读块缺少 group %s" % (path, group))
            continue
        for key in ("thresholdMaxMinRatio", "perfThresholdPct", "noiseFloorPct"):
            if key not in values:
                fail("%s 机读块 group %s 缺 %s" % (path, group, key))
        for key in ("thresholdMaxMinRatio", "perfThresholdPct"):
            if key in values and abs(values[key] - doc.get(key, -1)) > 1e-6:
                fail("%s group %s 的 %s=%.6f 与基线 %.6f 不一致"
                     % (path, group, key, values[key], doc.get(key, -1)))
        expect_floor = max((doc["thresholdMaxMinRatio"] - 1) * 100 * doc["noiseSafetyFactor"],
                           MIN_WINDOW_FLOOR_PCT)
        if "noiseFloorPct" in values and abs(values["noiseFloorPct"] - expect_floor) > 1e-4:
            fail("%s group %s 的 noiseFloorPct=%.4f 与基线推出的 %.4f 不一致"
                 % (path, group, values["noiseFloorPct"], expect_floor))


def parseCorpus(path):
    """解析 corpus.txt（<path>\\t<args>），返回用例列表并校验格式。"""
    cases = []
    with open(path, encoding="utf-8") as fh:
        for lineno, line in enumerate(fh, 1):
            line = line.rstrip("\n")
            if not line.strip() or line.lstrip().startswith("#"):
                continue
            parts = line.split("\t")
            if len(parts) != 2 or not parts[1].strip():
                fail("corpus.txt:%d 需要 <path>\\t<args>（args 不得为空）: %r" % (lineno, line))
                continue
            cases.append(dict(path=parts[0].strip(), args=parts[1].strip(), line=lineno))
    return cases


def parseExcluded(path):
    """解析 corpus-excluded.txt（<path>\\t(rc=<n>|k2-shape)\\t<原因>）。"""
    items = []
    with open(path, encoding="utf-8") as fh:
        for lineno, line in enumerate(fh, 1):
            line = line.rstrip("\n")
            if not line.strip() or line.lstrip().startswith("#"):
                continue
            parts = line.split("\t", 2)
            crit = parts[1].strip() if len(parts) > 1 else ""
            if len(parts) != 3 or not (re.fullmatch(r"rc=-?\d+", crit) or crit == "k2-shape"):
                fail("corpus-excluded.txt:%d 需要 <path>\\t(rc=<n>|k2-shape)\\t<原因>: %r" % (lineno, line))
                continue
            rc = int(crit[3:]) if crit.startswith("rc=") else None
            items.append(dict(path=parts[0].strip(), rc=rc, crit=crit, reason=parts[2].strip()))
    return items


def checkCorpus(root, corpusPath, excludedPath):
    """核对语料清单：路径存在、用例名唯一、两清单互斥、并集覆盖 cmd/circuits 下全部网表。"""
    if not os.path.isfile(corpusPath):
        fail("语料清单缺失: %s" % corpusPath)
        return
    if not os.path.isfile(excludedPath):
        fail("排除清单缺失: %s" % excludedPath)
        return
    cases = parseCorpus(corpusPath)
    excluded = parseExcluded(excludedPath)
    names = set()
    for case in cases:
        abs_path = os.path.join(root, case["path"])
        if not os.path.isfile(abs_path):
            fail("corpus.txt:%d 语料不存在: %s" % (case["line"], case["path"]))
        name = os.path.basename(case["path"])
        if name in names:
            fail("corpus.txt 用例名重复: %s" % name)
        names.add(name)
    exc_paths = set(i["path"] for i in excluded)
    for case in cases:
        if case["path"] in exc_paths:
            fail("语料 %s 同时在 corpus.txt 与 corpus-excluded.txt" % case["path"])
    all_nets = sorted(os.path.basename(p) for p in glob.glob(os.path.join(root, "cmd/circuits/*.net")))
    covered = set(os.path.basename(c["path"]) for c in cases) | set(os.path.basename(p) for p in exc_paths)
    missing = [n for n in all_nets if n not in covered]
    if missing:
        fail("cmd/circuits 下 %d 个网表未登记（corpus.txt 或 corpus-excluded.txt）: %s" % (len(missing), missing))
    notes.append("语料: 可用 %d 条 / 排除 %d 条 / 全部网表 %d 个" % (len(cases), len(excluded), len(all_nets)))


def main():
    """入口：跑完全部静态检查并打印每 group 摘要与 SELFCHECK 结论。"""
    parser = argparse.ArgumentParser(description="K1 静态自检")
    parser.add_argument("--root", required=True)
    parser.add_argument("--groups", required=True)
    parser.add_argument("--baselines", required=True)
    parser.add_argument("--thresholds", required=True)
    parser.add_argument("--corpus", required=True)
    parser.add_argument("--excluded", required=True)
    parser.add_argument("--require-baselines", action="store_true", help="基线缺失即失败（验收用）")
    args = parser.parse_args()

    rows = parseGroups(args.groups)
    baselines = {}
    print("%-4s %-10s %-18s %-12s %-8s %-7s %s" %
          ("grp", "benches", "thresholdMaxMinRatio", "noiseFloorPct", "perfPct", "runs", "commit"))
    for group in GROUPS:
        grows = [r for r in rows if r["group"] == group]
        path = os.path.join(args.baselines, group + ".json")
        if not os.path.isfile(path) and not args.require_baselines:
            fail("基线缺失: %s" % path)
            continue
        doc = checkBaseline(path, group, grows, args.root)
        baselines[group] = doc
        if doc is None:
            continue
        floor = (doc["thresholdMaxMinRatio"] - 1) * 100 * doc["noiseSafetyFactor"]
        floor = max(floor, MIN_WINDOW_FLOOR_PCT)
        print("%-4s %-10d %-18.4f %-12.2f %-8.1f %-7d %s" %
              (group, len(doc["entries"]), doc["thresholdMaxMinRatio"], floor,
               doc["perfThresholdPct"], doc["runs"], doc["commit"]))
    checkThresholdsDoc(args.thresholds, baselines)
    checkCorpus(args.root, args.corpus, args.excluded)
    history = sorted(glob.glob(os.path.join(args.baselines, "history", "*.json")))
    notes.append("基线历史: %d 个文件（%s）" % (len(history), os.path.join(args.baselines, "history")))
    for note in notes:
        print("NOTE: " + note)
    if errors:
        for msg in errors:
            print("FAIL: " + msg)
        print("SELFCHECK: FAIL (%d 项)" % len(errors))
        return 1
    print("SELFCHECK: PASS")
    return 0


if __name__ == "__main__":
    sys.exit(main())
