# 万级节点实现缺口清单（交接用）

- 创建: 2026-10-20
- 更新: 2026-10-21（宏模型落地、1e4/2.3e4 实测、klu 切换）
- 基线: CPU8 54/57 网表 2173/2192 节点；55/56 约 900 节点；58 宏模型 501 节点。

## 已完成

| 项 | 结果 | 证据 |
|---|---|---|
| klu 稀疏族移植 | 12 源文件 + 9 测试；解与旧 LU 一致（relErr ~1e-16） | `maths/klu*.go`；`TestKLUAB` |
| 默认入口切换 | `NewLUSparse` → klu；1e4 Poisson 分解 0.61s（旧 1.63s） | `maths/lu.go`、`maths/kluOptions.go` |
| 节点上限可配置 | `CIRCUIT_MAX_NODES`（默认仍 5000） | `load/load.go:548` |
| 1e4 RC 实测 | 101 步、97s、965ms/步；分解 40%、GetRow 2.6% | `TestRCScale1e4` |
| 2.3e4 门级 CPU 实测 | 17 步、166s、9.8s/步；分解 82%（5.5 次/步） | `TestCPU8Scale5x` |
| **58 宏模型（预置）** | 单份 497 元件/501 节点（原 2695/4567）；5 份每步 **404ms**（原 9.82s，24×） | `cmd/circuits/58_cpu8v2_macro.net`、`TestCPU8Macro*` |
| BTF 实测 | 门级矩阵更慢（17.1 vs 9.8s/步），默认关 | `CIRCUIT_KLU_BTF` |

## 进行中 / 卡点

| 项 | 状态 | 卡点 |
|---|---|---|
| 59 串口加载宏模型 | 加载成功 11/12 字节，最后一字节 `0xAA→0xAB`（D0 位） | RAM 写窗口对 D0 瞬时电平敏感；门级 SR 锁存器不敏感。pulse 2→3、去 `NoConverged` 均无效，已回滚 |
| 旧 `luSparse` 清理 | **已完成**（2026-10-21） | 删除旧 `luSparse`/`luSkeleton`/`luKernel.go`/`luSignature_test.go`/`lu_sparse_test.go`；`lu.go` 749→175 行，`rowSparse.go` 373→300 行（保留 `rowSparseMatrix` 给 benchmark）；maths 测试 + CPU8 回归通过 |

## 未做（按收益排序）

1. **59 的 RAM 写语义**：给 `logic.RAM` 加「WE 高且 D 连续两拍一致才写」或写时锁存前一拍 D；修好后跑 59 + 57 回归。
2. **宏模型扩到 55/56**：55 已是行为 RAM（894 节点），可继续把 regfile/ALU 行为化。
3. **T4 表驱动器件模型**（`element/semiconductor` 的 exp 查表）：未做，宏模型前置。
4. **T6 多速率调度**（`element/time/`）：未做，工程大。
5. **T7 电压限制/源步进**（`element/semiconductor` + `simulation.go`）：未做。
6. **T8 波形数据管理**：CLI 已有 `-nodes`/`-format`/`doublebuffer` 压缩；流式/分段未做。

## 工具/环境事实

- GOROOT 精简：`go tool` 只有 `asm/cgo/compile/cover/fix/link/preprofile/vet`，**无 pprof**；系统无 `perf`。
- 有 `kluStats`/`matrixStats` 插桩（分解次数/耗时、GetRow/矩阵操作计数），可用于替代 profile。
- 54b/58/59 测试需要 `CIRCUIT_GMCONT=1`、`CIRCUIT_GLOBALGMIN=1e-3` 等收敛环境，否则 t=0 不收敛。
- 网表字符串参数含 a-f 的十六进制必须用双引号字面量（`SplitTokens` 会把字母段切碎）。

## 2026-10-22 团队 P0–P3

| 项 | 状态 | 结果/证据 |
|---|---|---|
| P1 55/56 宏模型 | 完成 | `element/logic/alu.go`、`regFile.go`；60 网表 571 元件/563 节点（55 为 952/894，-37%）；R0..R3、RAM[14]、RAM 全量一致；`TestCPU8v3MacroNetlist` PASS |
| P2 T4 表驱动 | 完成 | `expTable.go` 默认关；最大相对误差 1.9e-6；exp 7.58→3.74 ns、二极管线性化 -30%；默认路径逐位不变 |
| P2 T7 电压限制 | 完成（核对） | `limitingBoundary_test.go` 覆盖 pnjlim 边界；与 SPICE3 差异仅记录，默认行为未改 |
| P3 T6 多速率 | 骨架完成，默认关 | `multiRate.go` + 开关；7 用例 PASS；无生产元件实现 MultiRateSteppable，本轮对 29/CPU8 收益 0 |
| P0 59 串口宏模型 | **未完成** | LOADER 收到最后一字节 `0xAB`（D0=5V），顶层 UART/LOADER 接线与 57 完全一致；pulse/NoConverged/MaxStep/端口改名 4 次无效；需 UART/LOADER 握手时序级诊断 |

集成验收：`go build ./...` OK；element/logic、element/semiconductor、element/time、maths、load 全绿；`TestCPU8v3Regfile|TestCPU8v2Preloaded` ok 275.9s；新文件 lint 0 违规 0 告警。
