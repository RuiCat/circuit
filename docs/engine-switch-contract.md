# 引擎开关契约（c6，engine-intelligence）

- 日期: 2026-10-24
- 目的: 冻结现有开关的“语义三元组（名 / 取值规则 / 绑定时刻）”，登记 `os.Getenv` 白名单；**旧语义只增不改**，新开关不再散落读取。
- 约束: C8（30-constraints.md）

## 旧 env 语义（三套，互不统一）

| 开关 | 取值规则 | 绑定时刻 | 读取位置 |
|---|---|---|---|
| `CIRCUIT_GMCONT` | **存在即真**（`=0` 也开启） | 每次仿真启动（`StrategyBinding`） | `load/load.go` → `element.EngineCfg.GminCont` |
| `CIRCUIT_GLOBALGMIN` / `CIRCUIT_GMINFINAL` / `CIRCUIT_NATGMIN` | 数值解析（非正回退默认） | 每次仿真启动 | `load/load.go` → `EngineCfg` |
| `CIRCUIT_MULTIRATE` | **值解析**：`""/0/false/off/no` 关；其他非空值（含未知值）开（`default: true`，不告警） | 仿真启动（`resolveMultiRateEnabled`） | `element/time/multiRateEnv.go` |
| `CIRCUIT_EXPTABLE` | **ParseBool**：`0/false` 关、非法值告警并保持原状态 | 进程内全局（`init`） | `element/semiconductor/expTable.go` |
| `CIRCUIT_KLU_BTF` / `CIRCUIT_KLU_DROP` | `1/true` 开；`ParseFloat` 非负 | 每次 `NewLUSparse`（新建求解器） | `maths/kluOptions.go` |

**不允许**把三套统一成一种规则：统一成“存在即真”会让 `CIRCUIT_MULTIRATE=0` 变成开；统一成 bool 会让 `CIRCUIT_GMCONT=0` 从开变关（破坏旧脚本）。新开关必须逐条声明语义三元组。

## os.Getenv 白名单

- `element/semiconductor/expTable.go`（`CIRCUIT_EXPTABLE`，进程级）
- `element/time/multiRateEnv.go`（`CIRCUIT_MULTIRATE`）
- `element/semiconductor/limiting.go`（`ITERATION_LIMIT_ENV`，既有迭代限制开关；登记为例外）
- `element/time/engineConfig.go`（迁移过渡：只读 `EngineCfg`，不直接读 env；登记为例外）
- `load/load.go`（`GMCONT`/`GLOBALGMIN`/`GMINFINAL`/`NATGMIN`/`LUSPARSE`/`LUSDENSE` 等 → `EngineCfg`）
- `maths/kluOptions.go`（`KLU_BTF`/`KLU_DROP`）

**禁止新增** `element/` 内的 `os.Getenv` 读取点；新开关走 `load.EngineCfg`（载入期）或 `TimeMNA` 显式配置（运行期）。

## 验证

```bash
grep -rn 'os.Getenv' element --include=*.go   # 仅白名单（expTable.go / multiRateEnv.go / engineConfig.go 迁移例外）
go test -count=1 ./element/time/ ./element/semiconductor/
```

## 已知边界

- `CIRCUIT_MULTIRATE` 与 `ParallelOpts` 非空显式互斥（`multiRate.go` 报错），不静默降级。
- 表驱动默认关（c4）；默认开需另立裁决并给电路级误差/回归。
- 多速率骨架默认关、`plan=nil` 走原路径；无生产元件（C9 降级）。
