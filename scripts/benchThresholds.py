#!/usr/bin/env python3
"""benchThresholds.py —— `baselines/thresholds.md` 的数值区生成与变更记录（K1/C10）。

用法：
    python3 scripts/benchThresholds.py sync         --baselines <dir> --thresholds <file>
    python3 scripts/benchThresholds.py log-change   --thresholds <file> --tsv <file>

设计：`baselines/*.json` 是**权威数值**，`thresholds.md` 的数值区由本脚本从 JSON 生成
（`--write-baseline` 写完基线后调用 sync），因此"重标定后立刻 --selfcheck"必然一致；
叙述性文字（口径、退出码语义、语料说明）在标记之外，本脚本永不改动。

标记区（缺一即报错退出 1，绝不静默改文本）：
    <!-- BENCH-GROUP-TABLE-BEGIN --> … <!-- BENCH-GROUP-TABLE-END -->
    <!-- BENCH-THRESHOLDS-BEGIN -->  … <!-- BENCH-THRESHOLDS-END -->   机读块（--selfcheck 读）
    <!-- BENCH-ENTRY-TABLE-BEGIN --> … <!-- BENCH-ENTRY-TABLE-END -->
    <!-- BENCH-CHANGELOG-BEGIN -->   … <!-- BENCH-CHANGELOG-END -->    变更记录（新行插在表头后）

log-change 的 TSV 每行（制表符分隔）：
    group  oldRatio  newRatio  historyFile  loadAvg  commit  reason

退出码：0=成功 / 1=错误（标记缺失、基线缺失、JSON 非法） / 2=用法错误。
"""

import argparse
import glob
import json
import os
import re
import sys

GROUPS = ["m1", "m2", "m3", "m4", "m5", "m6"]
# 与 maths/benchlib_test.go 的常量保持一致
MIN_WINDOW_FLOOR_PCT = 5.0
JUDGE_BAND = 1.5
DEFAULT_SAFETY = 2.0
DEFAULT_PERF_PCT = 5.0


class ThresholdError(Exception):
    """标记缺失 / 基线缺失 / JSON 非法。"""


def readBaselines(directory):
    docs = {}
    for group in GROUPS:
        path = os.path.join(directory, group + ".json")
        if not os.path.isfile(path):
            continue
        with open(path, encoding="utf-8") as fh:
            doc = json.load(fh)
        for field in ("group", "thresholdMaxMinRatio", "entries"):
            if field not in doc:
                raise ThresholdError("%s 缺字段 %s" % (path, field))
        doc["_path"] = path
        doc["_file"] = group + ".json"
        docs[group] = doc
    if not docs:
        raise ThresholdError("目录下没有任何 m*.json 基线: %s" % directory)
    return docs


def groupFloorPct(doc):
    """group 级噪声地板（%）= max over entries，下限 minWindowFloorPct。"""
    safety = doc.get("noiseSafetyFactor", DEFAULT_SAFETY)
    worst = max([(e["maxMinRatio"] - 1) * 100 * safety for e in doc["entries"]] or [0.0])
    return max(worst, MIN_WINDOW_FLOOR_PCT)


def failPct(doc, entry):
    """逐 entry 判定门槛（%）= max(perfThresholdPct, 2×entry 噪声地板)。"""
    safety = doc.get("noiseSafetyFactor", DEFAULT_SAFETY)
    floor = max((entry["maxMinRatio"] - 1) * 100 * safety, MIN_WINDOW_FLOOR_PCT)
    return max(doc.get("perfThresholdPct", DEFAULT_PERF_PCT), 2 * floor)


def renderGroupTable(docs):
    lines = ["| group | benchmark 数 | 包 | benchtime | 标定 thresholdMaxMinRatio | 噪声地板（%，下限 5） | 性能阈值（%） | 判定门槛（%） | 采集时 loadAvg | 标定时刻 |",
             "|---|---|---|---|---|---|---|---|---|---|"]
    for group in GROUPS:
        doc = docs.get(group)
        if doc is None:
            continue
        packages = "+".join(sorted({e["package"] for e in doc["entries"]}))
        floor = groupFloorPct(doc)
        lines.append("| %s | %d | %s | %s | %.4f | %.2f | %.2f | %.2f | %s | %s |" % (
            group, len(doc["entries"]), packages, doc.get("benchtime", ""),
            doc["thresholdMaxMinRatio"], floor,
            doc.get("perfThresholdPct", DEFAULT_PERF_PCT), max(doc.get("perfThresholdPct", DEFAULT_PERF_PCT), 2 * floor),
            doc.get("loadAvg", ""), doc.get("generatedAt", "")))
    return "\n".join(lines)


def renderMachineBlock(docs):
    lines = []
    for group in GROUPS:
        doc = docs.get(group)
        if doc is None:
            continue
        lines.append("%s thresholdMaxMinRatio=%.9f perfThresholdPct=%.4f noiseFloorPct=%.4f" % (
            group, doc["thresholdMaxMinRatio"], doc.get("perfThresholdPct", DEFAULT_PERF_PCT), groupFloorPct(doc)))
    return "\n".join(lines)


def renderEntryTable(docs):
    lines = ["| group | benchmark | ns/op 三次样本 | maxMinRatio | 噪声地板 | 判定门槛 |",
             "|---|---|---|---|---|---|"]
    for group in GROUPS:
        doc = docs.get(group)
        if doc is None:
            continue
        safety = doc.get("noiseSafetyFactor", DEFAULT_SAFETY)
        for entry in sorted(doc["entries"], key=lambda e: e["name"]):
            floor = max((entry["maxMinRatio"] - 1) * 100 * safety, MIN_WINDOW_FLOOR_PCT)
            lines.append("| %s | %s | %s | %.4f | %.2f | %.2f |" % (
                group, entry["name"], " / ".join("%.6g" % v for v in entry["nsPerOp"]),
                entry["maxMinRatio"], floor, failPct(doc, entry)))
    return "\n".join(lines)


def replaceRegion(text, begin, end, body, path):
    if text.count(begin) != 1 or text.count(end) != 1:
        raise ThresholdError("%s 里标记 %s / %s 必须各出现一次" % (path, begin, end))
    pattern = re.compile(re.escape(begin) + r".*?" + re.escape(end), re.S)
    return pattern.sub(lambda _m: begin + "\n" + body.rstrip("\n") + "\n" + end, text, count=1)


def writeAtomic(path, data):
    tmp = path + ".tmp"
    with open(tmp, "w", encoding="utf-8") as fh:
        fh.write(data)
    os.replace(tmp, path)


def cmdSync(args):
    docs = readBaselines(args.baselines)
    if not os.path.isfile(args.thresholds):
        raise ThresholdError("阈值文档不存在: %s" % args.thresholds)
    with open(args.thresholds, encoding="utf-8") as fh:
        text = fh.read()
    text = replaceRegion(text, "<!-- BENCH-GROUP-TABLE-BEGIN -->", "<!-- BENCH-GROUP-TABLE-END -->",
                         renderGroupTable(docs), args.thresholds)
    text = replaceRegion(text, "<!-- BENCH-THRESHOLDS-BEGIN -->", "<!-- BENCH-THRESHOLDS-END -->",
                         renderMachineBlock(docs), args.thresholds)
    text = replaceRegion(text, "<!-- BENCH-ENTRY-TABLE-BEGIN -->", "<!-- BENCH-ENTRY-TABLE-END -->",
                         renderEntryTable(docs), args.thresholds)
    writeAtomic(args.thresholds, text)
    print("benchThresholds: 已按 %d 个基线同步 %s" % (len(docs), args.thresholds))
    return 0


def parseChanges(path):
    changes = []
    with open(path, encoding="utf-8") as fh:
        for lineno, line in enumerate(fh, 1):
            line = line.rstrip("\n")
            if not line.strip() or line.lstrip().startswith("#"):
                continue
            parts = line.split("\t")
            if len(parts) < 7:
                raise ThresholdError("%s:%d 需要 7 列（group/old/new/history/loadAvg/commit/reason）" % (path, lineno))
            changes.append(parts[:7])
    return changes


def cmdLogChange(args):
    changes = parseChanges(args.tsv)
    if not changes:
        print("benchThresholds: 无变更行，跳过")
        return 0
    if not os.path.isfile(args.thresholds):
        raise ThresholdError("阈值文档不存在: %s" % args.thresholds)
    with open(args.thresholds, encoding="utf-8") as fh:
        text = fh.read()
    begin, end = "<!-- BENCH-CHANGELOG-BEGIN -->", "<!-- BENCH-CHANGELOG-END -->"
    if text.count(begin) != 1 or text.count(end) != 1:
        raise ThresholdError("%s 里标记 %s / %s 必须各出现一次" % (args.thresholds, begin, end))
    body = text.split(begin, 1)[1].split(end, 1)[0].strip("\n").splitlines()
    if len(body) < 2:
        raise ThresholdError("%s 的变更记录区缺少表头/分隔行" % args.thresholds)
    rows = []
    for group, old, new, history, loadavg, commit, reason in changes:
        rows.append("| %s | %s | %s | %s | %s | %s | %s |" % (
            args.date, group, old, new, history or "（首次写入，无历史）", commit, "%s（loadAvg=%s）" % (reason, loadavg)))
    merged = body[:2] + rows + body[2:]
    text = replaceRegion(text, begin, end, "\n".join(merged), args.thresholds)
    writeAtomic(args.thresholds, text)
    print("benchThresholds: 已追加 %d 条变更记录到 %s" % (len(rows), args.thresholds))
    return 0


def main():
    parser = argparse.ArgumentParser(description="thresholds.md 数值区生成与变更记录")
    sub = parser.add_subparsers(dest="mode", required=True)

    sync = sub.add_parser("sync", help="按 baselines/*.json 重新生成数值区")
    sync.add_argument("--baselines", required=True)
    sync.add_argument("--thresholds", required=True)
    sync.set_defaults(func=cmdSync)

    log = sub.add_parser("log-change", help="向变更记录区插入新行（新行在前）")
    log.add_argument("--thresholds", required=True)
    log.add_argument("--tsv", required=True)
    log.add_argument("--date", default="")
    log.set_defaults(func=cmdLogChange)

    args = parser.parse_args()
    if args.mode == "log-change" and not getattr(args, "date", ""):
        import datetime
        args.date = datetime.date.today().isoformat()
    try:
        return args.func(args)
    except ThresholdError as exc:
        print("benchThresholds: ERROR: %s" % exc, file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
