# 多速率调度（c3，engine-intelligence）

- 日期: 2026-10-24
- 范围: P3 交付的调度骨架收口；**本轮不承诺生产元件接入**（C9 降级，用户 D2 裁决）。
- 实现: `element/time/multiRate.go`、`multiRateConfig.go`、`multiRateEnv.go`

## 设计

- **全局时间网格不变**：所有元件仍在同一接受步网格上；慢组在“宏边界”（`全局接受步序号 % stride == 0`）才 `DoStep`，其余步只 `HoldStep` 重驱动输出。
- **快慢组**：实现 `MultiRateSteppable`（`MultiRateStride() int` + `HoldStep`）的元件为慢组；未实现或倍率 ≤1 为快组。
- **对齐**：步序号按 `CurrentTime` 变化惰性递增，拒绝步重试同一时刻不重复计数；慢组推进时刻必落在某个全局接受步的求解时刻。
- **回滚/收敛**：非宏步不调 `DoStep`（不推进状态），改调 `HoldStep` 只重驱动输出（A/Z 每次迭代回滚到线性加盖状态，跳过 HoldStep 会让输出源 RHS 退回初值）。不改收敛判据/Newton/LU。
- **默认零影响**：未启用或无慢元件 → `plan=nil` → 原 `CallMark`/`ParallelCallMark` 路径逐字不变。

## 开关（语义三元组）

| 开关 | 取值规则 | 绑定时刻 |
|---|---|---|
| `CIRCUIT_MULTIRATE` | 值解析：`1/true/on/yes` 开；`""/0/false/off/no` 关；其他值告警保持关 | 仿真启动（`resolveMultiRateEnabled`） |
| `TimeMNA.EnableMultiRate()` / `DisableMultiRate()` | 显式 Context 配置，优先于 env | 下次 `TransientSimulation` 启动 |

## 已知边界（C9 降级）

- **全局步数不下降**：本骨架只省慢组 `DoStep` 调用数；真正的大步长需要矩阵分块（BTF）后对块单独积分，属后续阶段。
- **无生产元件实现 `MultiRateSteppable`**：`grep -rn 'func .*MultiRateStride' element --include=*.go | grep -v _test.go` 期望为 0；当前只有测试元件 `mrTestElement`。
- **收益 0**：对 29/CPU8 默认路径无影响；开启也无生产收益，故不作为性能证据。
- **并行互斥**：`ParallelOpts` 非空时显式报错，不静默降级。

## 验收

```bash
go test -count=1 ./element/time/
# 日志应含：steps 两场景相等、慢组推进时刻对齐、并行+多速率显式报错
```
