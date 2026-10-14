"""benchThresholdsBootstrap.py —— 首次标定时生成 thresholds.md 骨架（K1 引导）。

背景：`thresholds.md` 的数值区由 benchThresholds.py 从 baselines/*.json 生成，
但它要求文档已存在（缺标记区即报错）。全新克隆里 `runs/` 不存在，首次
`--write-baseline` 会因此跳过文档同步 —— 本模块补上"从零生成骨架"这一步。

骨架只含四对标记区与固定叙述；生成后立即由 benchThresholds.py sync 填数值。
"""
import io
import os
import sys

TEMPLATE = """# 基准阈值记录（K1）——「性能阈值」与「噪声地板」分列

> 本文件由 `scripts/benchThresholds.py` 生成/同步数值区；叙述性文字不会被自动改写。
> 数值权威来源是 `baselines/<group>.json`（本文件是它的可读投影）。

## 1. 判定口径（逐字）

- 每个 benchmark 取同 commit 同口径 **3 次运行的中位数**；
- `noiseFloorPct` = `max(标定窗口每 entry 的 maxMinRatio−1, 本次窗口的 maxMinRatio−1)` 的最大值，
  下限 5%；判定带为 **1.5× 噪声地板**（`benchlibJudgeBand=1.5`）：`|deltaPct| ≤ 1.5×floor` → `judgeable=false`；
- 失败门槛 `failThresholdPct = max(perfThresholdPct, 2×噪声地板)`：`judgeable` 且 `deltaPct > 失败门槛` → FAIL；
- `deltaPct = (baseline − current) / baseline × 100`，**负值 = 更快**。

> 代价（须知）：有效灵敏度 = 1.5× 地板。地板 13% 的条目需 >19.5% 的 delta 才「可判」，
> 中等真实改善（−15%~−34%）在高地板窗口会被算作不可判 —— delta 仍打印在判定表里，不得读成"无改善"。

## 1.1 公共模式漂移 → 整组降级（D29）

`environmentShiftPct`（各 entry deltaPct 的中位数）**显著且各条目围绕它聚集**时
（`|中位| > shiftLimit` 且 `极差 ≤ max(shiftLimit, 0.5×|中位|)`），判定为跨窗口漂移 →
`verdict=NOT_JUDGEABLE`（退出码 3），并写 `verdictBeforeShift`（降级前判定）与
`pendingFails`（疑似回归条目，不静默吞掉）。不聚集则保留逐条判定，避免把单点真回归
（如 `{+40%, +0.4%}`）误当漂移。策略类改动请用**同窗口受控交错 A/B** 作为主证据。

## 2. 退出码语义（单个 group）

`0=PASS`（judged>0 且无 FAIL）/ `1=FAIL` / `2=用法或基线缺失` / `3=给不出判定`
（compare 时 `judged==0` 即 D14，或检测到公共模式漂移即 D29；`--write-baseline` 时 `maxMinRatio > 1.15` 拒绝写入）。
`--group all` 取最严重者。**同代码对自身基线自比得到 rc=3 是预期语义**（delta 落在判定带内）。

## 3. 标定结果（数值区由 `benchThresholds.py sync` 生成）

<!-- BENCH-GROUP-TABLE-BEGIN -->
<!-- BENCH-GROUP-TABLE-END -->

<!-- BENCH-THRESHOLDS-BEGIN -->
<!-- BENCH-THRESHOLDS-END -->

## 4. 每 benchmark 的标定数据（数值区由 `benchThresholds.py sync` 生成）

<!-- BENCH-ENTRY-TABLE-BEGIN -->
<!-- BENCH-ENTRY-TABLE-END -->

## 5. 基线历史（可复现性）

每次 `--write-baseline` 会把旧基线原子备份到 `baselines/history/<group>-<generatedAt>.json`，
并在新基线的 `previousBaseline` 字段里链接它 —— 因此"某次收益声明"始终可用当时的基线复现：
`bash scripts/benchCompare.sh --group m2 --baseline runs/circuit-perf-optimization/baselines/history/m2-<时间>.json`

## 6. 语料（C15）

可用语料见 `scripts/corpus.txt`（仓库内，显式列举，每条先过可用性判据）；
不可用见 `scripts/corpus-excluded.txt`（`rc=<n>` 或 `k2-shape` 判据）。
`runs/circuit-perf-optimization/baselines/` 下的同名副本优先（本机工作副本）。

## 7. 复算命令

```bash
export GOCACHE=$PWD/.cache/gocache GOPATH=$PWD/.cache/gopath GOMODCACHE=$PWD/.cache/go-mod
bash scripts/benchCompare.sh --group all --write-baseline   # 首次使用：标定本机基线
bash scripts/benchCompare.sh --group all --selfcheck        # 名单/基线/阈值/语料一致性
bash scripts/benchCompare.sh --group all                    # 判定（0/1/2/3）
bash scripts/regressSnapshot.sh scripts/corpus.txt .cache/snap
python3 scripts/regressDiff.py .cache/snap-before .cache/snap --rel-tol 1e-6
```

## 8. 阈值变更记录（新行由 `--write-baseline` 自动插入；C10：旧值 / 新值 / 触发模块）

<!-- BENCH-CHANGELOG-BEGIN -->
| 日期 | group | 旧值 | 新值 | 历史基线 | commit | 触发原因（loadAvg） |
|---|---|---|---|---|---|---|
<!-- BENCH-CHANGELOG-END -->
"""


def ensure(path: str) -> str:
    """若 thresholds.md 不存在则生成骨架；返回 'created' 或 'exists'。"""
    if os.path.exists(path):
        return "exists"
    parent = os.path.dirname(path)
    if parent:
        os.makedirs(parent, exist_ok=True)
    tmp = path + ".tmp"
    with io.open(tmp, "w", encoding="utf-8") as f:
        f.write(TEMPLATE)
    os.replace(tmp, path)
    return "created"


if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("用法: benchThresholdsBootstrap.py <thresholds.md>", file=sys.stderr)
        raise SystemExit(2)
    print(ensure(sys.argv[1]))
