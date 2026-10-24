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

## 真实对比结果（2026-10-24，circuit vs ngspice-47）

| 用例 | 节点 | circuit | ngspice | rel | verdict |
|---|---|---|---|---|---|
| 01_divider（DC 线性） | 2 | 3.3333333 | 3.333333 | 1.0e-7 | PASS |
| 02_rc（RC 瞬态，t=0.1ms，uic 初值 0） | 2 | 0.47807 | 0.47581 | 4.75e-3 | **FAIL** |
| 07_rlc（RLC 瞬态，t=0.1ms，uic 初值 0） | 3 | 3.15310 | 3.14441 | 2.76e-3 | **FAIL** |

命令：`bash scripts/benchAccuracy/compare.sh`（任一 rel>1e-3 时 exit 1）。
结论：DC 线性一致；**瞬态（RC/RLC）与 ngspice 差 0.3–0.5%，未达 1e-3**——积分器/误差控制/初值处理存在差异，需后续对齐（C3 降级，见 40-plan）。

## 瞬态误差根因实验（2026-10-24，RC）

- 容差/步长：`--min-step 1e-12 --max-step 1e-9` 下，`--rel-tol` 1e-3/1e-4/1e-5、`--abs-tol` 1e-12/1e-6/1e-9 的末值**均为 0.4780739（不变）**；ngspice 为 0.4758129。
- 解析解 5(1-e^-0.1)=0.475813（ngspice 与解析解一致，circuit 偏高 0.47%）。
- 结论：0.47% 偏差来自**积分器/局部截断误差控制**（非步长/容差），需升级积分器（TRAP/GEAR）或与 ngspice 对齐；C3 保持降级。

## 已知边界（本轮最小闭环）

- 语料 10 条（不是 ≥20）；误差对比循环与 golden 输出冻结留待 m5 前补齐。
- circuit 侧 CLI 输出需与参考器按同一时间点对齐；对齐/插值规则在 metric 实现中定义（C3）。
