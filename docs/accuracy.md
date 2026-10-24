# 外部精度基准（m1 / c1）

- 参考器: `third_party/reference/ngspice`（ngspice-47，sha256 见 `SHA256SUMS`）
- 语料: `cmd/circuits/acc-corpus.txt`（当前 10 条，后续补到 ≥20 条并冻结 sha256）
- harness: `scripts/benchAccuracy/run.sh --selfcheck`
- 判据: 每条用例逐节点 `|a-b| ≤ atol + rtol·|b|`（atol=1e-9，rtol=1e-3），近零节点用 atol；AC 分幅值/相位；噪声用 PSD；缺失参考器 → `exit 3`（BLOCKED，不得 PASS）。

## 命令

```bash
bash scripts/benchAccuracy/run.sh --selfcheck
# 期望: {"ok":true,...,"version":"ngspice-47","sha256":"...","corpus":"..."} 且 exit 0
# 参考器缺失/校验失败: {"blocked":true,...} 且 exit 3
```

## 已知边界（本轮最小闭环）

- 语料 10 条（不是 ≥20）；误差对比循环与 golden 输出冻结留待 m5 前补齐。
- circuit 侧 CLI 输出需与参考器按同一时间点对齐；对齐/插值规则在 metric 实现中定义（C3）。
