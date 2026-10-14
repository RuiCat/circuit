#!/usr/bin/env python3
"""regressDiff.py —— K2：前后快照对比（相对误差 / 逐位一致）。

用法：
    python3 scripts/regressDiff.py <before> <after> [--rel-tol 1e-6] [--bit-exact]

输入：两个目录，各含 regressSnapshot.sh 产出的 <case>.csv（`%.17g`）。
输出（stdout，JSON）：
    {"maxAbsDiff":..,"maxRelDiff":..,"bitExact":..,"cases":N,"verdict":"PASS|FAIL",
     "onlyBefore":[..],"onlyAfter":[..],"perCase":[{name,maxAbsDiff,maxRelDiff,bitExact,rows}]}

判定：
    - case 集合不一致 → FAIL 并列出差集（不会只比交集）；
    - 表头/行数/列数不一致 → FAIL；
    - 出现 NaN/Inf → FAIL；
    - `--bit-exact`：要求逐位一致（float64 位模式相等）；
    - 否则：要求 maxRelDiff ≤ --rel-tol；相对误差口径与仓库既有约定一致
      （`maths/lu_sparse_test.go:115-123`：|after-before| / max(1.0, |before|)）；
    - 两侧 0 个用例 → FAIL（禁止空集"通过"）。

退出码：0=PASS / 1=FAIL / 2=用法或输入缺失。
"""

import argparse
import glob
import json
import math
import os
import struct
import sys


def loadCase(path):
    """读取一份快照 CSV，返回 (表头行, float 二维表)；单元格非数值即抛 ValueError。"""
    """读取快照：返回 (header, rows)；rows 为 float 二维表。"""
    with open(path, encoding="utf-8") as fh:
        lines = fh.read().splitlines()
    if not lines:
        raise ValueError("空文件: %s" % path)
    header = lines[0]
    rows = []
    for lineno, line in enumerate(lines[1:], start=2):
        values = []
        for cell in line.split(","):
            try:
                values.append(float(cell))
            except ValueError:
                raise ValueError("%s:%d 非数值单元格 %r" % (path, lineno, cell))
        rows.append(values)
    return header, rows


def bits(value):
    """返回 float64 的 IEEE-754 位模式（逐位比较用）。"""
    return struct.pack(">d", value)


def main():
    """入口：比对两个快照目录，输出 K2 约定的 JSON 并按 verdict 返回退出码。"""
    parser = argparse.ArgumentParser(description="K2 快照对比")
    parser.add_argument("before")
    parser.add_argument("after")
    parser.add_argument("--rel-tol", type=float, default=1e-6)
    parser.add_argument("--bit-exact", action="store_true")
    args = parser.parse_args()

    for label, path in (("before", args.before), ("after", args.after)):
        if not os.path.isdir(path):
            print("输入缺失: %s 目录不存在: %s" % (label, path), file=sys.stderr)
            return 2

    def cases(directory):
        return sorted(os.path.basename(p) for p in glob.glob(os.path.join(directory, "*.csv")))

    before, after = cases(args.before), cases(args.after)
    only_before = [c for c in before if c not in set(after)]
    only_after = [c for c in after if c not in set(before)]
    common = [c for c in before if c in set(after)]

    doc = {
        "maxAbsDiff": 0.0,
        "maxRelDiff": 0.0,
        "bitExact": True,
        "cases": len(common),
        "verdict": "PASS",
        "onlyBefore": only_before,
        "onlyAfter": only_after,
        "perCase": [],
    }
    failures = []

    if only_before or only_after:
        failures.append("case 集合不一致: before 独有 %s / after 独有 %s" % (only_before, only_after))
    if not common:
        failures.append("没有可比较的用例（两侧 *.csv 交集为空）")
    if failures:
        doc["verdict"] = "FAIL"
        doc["notes"] = failures
        print(json.dumps(doc, ensure_ascii=False, indent=2))
        return 1

    for name in common:
        try:
            header_b, rows_b = loadCase(os.path.join(args.before, name))
            header_a, rows_a = loadCase(os.path.join(args.after, name))
        except ValueError as exc:
            failures.append(str(exc))
            continue
        case = {"name": name, "maxAbsDiff": 0.0, "maxRelDiff": 0.0, "bitExact": True, "rows": len(rows_b)}
        if header_b != header_a:
            failures.append("%s 表头不一致: %r vs %r" % (name, header_b, header_a))
            case["bitExact"] = False
            doc["perCase"].append(case)
            continue
        if len(rows_b) != len(rows_a):
            failures.append("%s 行数不一致: %d vs %d" % (name, len(rows_b), len(rows_a)))
            case["bitExact"] = False
            doc["perCase"].append(case)
            continue
        for index, (row_b, row_a) in enumerate(zip(rows_b, rows_a), start=2):
            if len(row_b) != len(row_a):
                failures.append("%s:%d 列数不一致: %d vs %d" % (name, index, len(row_b), len(row_a)))
                case["bitExact"] = False
                break
            for col, (value_b, value_a) in enumerate(zip(row_b, row_a)):
                if not (math.isfinite(value_b) and math.isfinite(value_a)):
                    failures.append("%s:%d:%d 出现 NaN/Inf: %r vs %r"
                                    % (name, index, col, value_b, value_a))
                    case["bitExact"] = False
                    continue
                if bits(value_b) != bits(value_a):
                    case["bitExact"] = False
                abs_diff = abs(value_a - value_b)
                rel_diff = abs_diff / max(1.0, abs(value_b))
                case["maxAbsDiff"] = max(case["maxAbsDiff"], abs_diff)
                case["maxRelDiff"] = max(case["maxRelDiff"], rel_diff)
                doc["maxAbsDiff"] = max(doc["maxAbsDiff"], abs_diff)
                doc["maxRelDiff"] = max(doc["maxRelDiff"], rel_diff)
        if not case["bitExact"]:
            doc["bitExact"] = False
        if not case.get("bitExact", True) and args.bit_exact:
            failures.append("%s 不是逐位一致" % name)
        doc["perCase"].append(case)

    if args.bit_exact:
        if not doc["bitExact"]:
            failures.append("--bit-exact 要求逐位一致，实测 maxAbsDiff=%g" % doc["maxAbsDiff"])
    elif doc["maxRelDiff"] > args.rel_tol:
        failures.append("maxRelDiff=%g > --rel-tol=%g" % (doc["maxRelDiff"], args.rel_tol))

    if failures:
        doc["verdict"] = "FAIL"
        doc["notes"] = failures
        print(json.dumps(doc, ensure_ascii=False, indent=2))
        return 1
    print(json.dumps(doc, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
