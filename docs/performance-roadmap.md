# 大规模电路仿真加速实现规划（参考商业仿真器架构）

> 状态：规划草案（2026-08-22）
> 参考对象：KLU/SuiteSparse（稀疏直接法）、SPICE3（Markowitz/限制）、HSPICE/Spectre（Gmin stepping/源步进）、FastSPICE 系 FineSim/XA/XPS（划分/多速率/宏模型）
> 现状基线：CPU8（2210 节点 RTL CPU）4.5ms 瞬态 ≈106s（229 步，~0.46s/步），每步 = 一次完整 LU 分解 + 牛顿迭代。

---

## 阶段 0：gmin 语义修正收尾（当前未提交改动，建议先提交）

**参考**：SPICE 标准——continuation gmin（Gmin stepping 阻尼）只加在非线性元件 PN 结上；全局 gmin 是独立小常数（默认 ~1e-12 S），仅消除无直流通路的病态节点。

**改动**（已在工作区）：
- `element/time/simulation.go`：`CIRCUIT_GLOBALGMIN` 支持显式数值（默认 1e-9 S=1GΩ）；`gminForLU` 不再叠加 continuationGmin。
- 实测：cnt3v4 有/无 GLOBALGMIN 均完美计数 0→1→2→3→4→5→6（修复前 GLOBALGMIN 冻结）。
- CPU8 实验（`--max-step 2e-4`，2ms 目标）：

| GLOBALGMIN | 完成度 | 强制推进次数 |
|---|---|---|
| 1e-9（默认） | ~39% @600s 超时 | 每步（~8s/步） |
| 1e-7 | 39% | 37 |
| **1e-5** | **100%** | **4** |
| 1e-4 | 39% | 77 |

**结论**：1e-5 是甜点——足够给弱连接节点接地（避免近奇异），又远小于 RTL 门上拉电导 1e-3 S（不冻死逻辑）。默认值建议从 1e-9 提升到 1e-7~1e-5 区间或文档注明 CPU8 需 `CIRCUIT_GLOBALGMIN=1e-5`。

**验收**：29 网表回归 + cnt3v4 计数 + CPU8 2ms 完成且状态机翻转。

---

## 阶段 1：符号/数值分解分离（KLU 模式）——最高 ROI

**参考**：KLU（SuiteSparse）核心流程 `klu_analyze`（符号分解，确定 fill-in 模式，只做一次）→ `klu_factor`（数值分解，每步复用结构只更新值）→ `klu_refactor`。矩阵结构不变时（我们每步都是），符号分解完全可复用。

**现状**：`perm` 已按结构签名缓存（605d8cc），但 `decomposeWithPerm` 每步仍重建 U/L 行列结构（rowSparse 动态插入 fill）。

**实现**：
1. `maths/lu.go`：新增 `symbolicFactor(perm)` 阶段——按 perm 预计算 L/U 的列结构（fill pattern），产出 CSR/CSC 骨架。
2. `rowSparse` 增加"骨架预分配"模式：数值阶段只写值，不重新分配/插入。
3. `decomposeWithPerm` 拆为 `Symbolic()` + `Numeric()`，`permSig` 命中时跳过 Symbolic。
4. 结构变化（元件通断改变连接）时自动失效重做。

**收益**：每步省去 fill-in 结构构建 + 内存分配。CPU8 每步 0.46s 中 LU 占大头（此前 EquilibrateAndDecompose 是 pprof 热点），预计每步快 2~4 倍。

**验收**：29 网表稠密/稀疏逐位一致；CPU8 每步耗时显著下降；pprof 确认 LU 数值部分占比。

---

## 阶段 2：动态 Markowitz 主元（SPICE3 模式）——fill-in 根治

**参考**：SPICE3 消元时按 Markowitz 准则选主元（最小 fill-in × 数值安全阈值组合），每步在剩余子矩阵中局部扫描。现代实现：静态预排序（AMD/最小度）+ 动态部分主元（阈值选主）。

**现状**：静态最小度重排（`maths/ordering.go`）+ 奇异回退（重排失败回自然序）——依赖"重排对了就快"的脆性，CPU8 靠 hub 判据（maxDeg>10×avgDeg）碰对。

**实现**：
1. 消元过程维护剩余列的最小 fill 计数，选主元时综合数值阈值（|a_kk| ≥ tol·max|a_ik|）与 fill 代价。
2. 静态重排只做初始近似，动态 Markowitz 作为局部修正（限制扫描窗口避免 O(n²) 开销）。
3. 移除/弱化"奇异回退"逻辑（回退是重排不优的症状，Markowitz 根治后不需要）。

**收益**：fill-in 控制从启发式变算法保证；不再依赖网表结构判据；任意拓扑稳健。

**验收**：构造 fill-in 最坏情况矩阵（如 3D 网格 + hub 混合）对比 fill 数与分解时间；29 网表回归；CPU8 fill-in 数不回升。

---

## 阶段 3：牛顿迭代增强——电压限制与源步进（HSPICE/Spectre 模式）

**参考**：
- **电压限制（voltage limiting）**：每牛顿迭代限制节点电压变化 ≤ 0.2~0.3V（BJT/MOS 按 Vt 缩放），防止指数模型发散。SPICE3 `nl_limiting`，Spectre 阻尼牛顿。
- **源步进（source stepping）**：把电源从 0 步进到目标值，每级牛顿收敛后再升——比 Gmin stepping 更适合含电源大摆幅电路。

**现状**：Gmin stepping + `gminStuck` 检测 + force-advance（CPU8 每步仍偶发 4 次强制推进，说明收敛策略未完善）。

**实现**：
1. `element/semiconductor/`：BJT/MOS 迭代电压限制（按 Vbe/Vgs 限制步进，标准公式）。
2. 强制推进前先尝试电压限制（而不是直接接受超限解）。
3. 可选：`CIRCUIT_SOURCESTEP=1` 源步进路径（与 Gmin 二选一）。

**收益**：force-advance 归零；大摆幅/边沿敏感电路（CPU8 锁存器）牛顿收敛更稳。

**验收**：CPU8 全配置 0 强制推进；03 整流器等非线性网表输出与理论一致。

---

## 阶段 4：非线性器件表驱动模型（FastSPICE 模式）

**参考**：FastSPICE/HSPICE 表模型——预计算 exp/双曲函数到查找表（对数坐标），迭代时查表 + 线性插值，避免每步指数求值。

**现状**：`element/semiconductor/` 每步算 `exp(V/Vt)` 等。

**实现**：
1. BJT Ebers-Moll / MOS Shichman-Hodges 的 I(V)、g(V) 预计算查表（电压范围 -5V~+5V，~2^16 点）。
2. 表驱动版与解析版输出逐位一致（容差内）验证后切换。

**收益**：非线性求值提速 5~20 倍（CPU8 数千晶体管每步多次求值）；对阶段 5 宏模型也是前置依赖。

**验收**：表/解析两版在 29 网表上 maxdiff < 1e-9；基准测试（pprof）确认求值占比下降。

---

## 阶段 5：层次化宏模型（FastSPICE 模式）——数量级收益

**参考**：FastSPICE 对标准单元/子电路预求解宏模型——输入电压→输出电流/电压的查表 + 输出阻抗，仿真时整门替换为宏模型，节点数骤减。

**现状**：子电路全部展开成晶体管级（CPU8 展开后 2210 节点）。`rtl_nor`/`msff`/`ramcell` 等标准门反复出现（313 顶层实例）。

**实现**：
1. 网表加载后识别**重复子电路实例**（相同 subckt 拓扑 + 相同参数）。
2. 对每个唯一门做一次 DC 传输特性求解（输入组合 × 电源电压），存成宏模型（多输入→输出查表 + 输出电阻 + 输入电容）。
3. 展开时同类门共享宏模型（或仿真时按宏模型求值，不再逐晶体管 stamp）。
4. 验证：宏模型版与晶体管版在 09 半加器/cnt3v4/CPU8 输出一致（逻辑级逐位，模拟级容差）。

**收益**：CPU8 节点 2210 → 数百，每步规模下降一个数量级；配合阶段 1 后 CPU8 可能从分钟级到秒级。

**验收**：cnt3v4 计数、CPU8 状态机波形与晶体管版一致；CPU8 仿真墙钟时间大幅下降。

---

## 阶段 6：电路划分多速率（FineSim/XA 模式）——远期

**参考**：FastSPICE 把电路按 CCC（沟道连通分量）划分，块内小步长精确求解、块间大步长 + 事件驱动求值；数字区与模拟区分离速率。

**现状**：全局统一步长（每步全矩阵 LU）。

**实现**：先做**结构划分**（强连通块检测，参考 KLU BTF 块三角化），再做多速率调度。工程量大，放最后。

**验收**：CPU8 数字部分大步长、模拟部分（如有）小步长；总步数下降且波形一致。

---

## 阶段 7：并行化（KLU 多线程 / SuperLU_DIST 模式）——远期

**参考**：多线程稀疏 LU（SuperLU_MT/KLU 多线程版）、分块并行（SuperLU_DIST）。

**实现**：`maths/parallel.go` 已有雏形；LU 数值分解按列分块并行，或右端项多路并行（同矩阵多 b 时）。

---

## 优先级总览

| 阶段 | 内容 | 参考 | ROI | 依赖 | 工作量 |
|---|---|---|---|---|---|
| 0 | gmin 语义修正收尾 | SPICE 标准 | 修复正确性 | — | 已基本完成 |
| 1 | 符号/数值分解分离 | KLU | ★★★★ | 无 | 中 |
| 2 | 动态 Markowitz | SPICE3 | ★★★★ | 无 | 中 |
| 3 | 电压限制/源步进 | HSPICE/Spectre | ★★★ | 无 | 中 |
| 4 | 器件表驱动模型 | FastSPICE | ★★★ | 无 | 小-中 |
| 5 | 层次化宏模型 | FastSPICE | ★★★★★ | 4 | 大 |
| 6 | 电路划分多速率 | FineSim/XA | ★★★★ | 5 | 很大 |
| 7 | 并行 LU | SuperLU | ★★★ | 1 | 中 |

**建议路线**：0 → 1 → 2 → 3（数值内核四连）→ 4 → 5（数量级跳变）→ 6 → 7。
每一阶段独立可提交、独立可回归（29 网表 + CPU8 双基线）。
