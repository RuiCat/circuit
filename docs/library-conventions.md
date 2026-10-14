# 库约定：类型能力矩阵、开关绑定时刻表与扩展点

本文把三件"代码里只有隐式约定"的事写成显式契约：**哪个数值类型能在哪条求解路径上求解**、
**每个开关在哪一刻绑定**、**新增元件/物理域/求解后端必须接进哪个口**。

代码是唯一权威；本文与代码不一致时以代码为准，并应立刻修本文。对应实现位置：

| 契约 | 实现位置 | 机检入口 |
|---|---|---|
| 类型能力矩阵 | `mna/typecapability.go` | `go test -count=1 -run 'TypeCapability' ./mna/` |
| 后端绑定的载入期不可变 | `element/context.go`（`BackendBinding`）、`element/time/engineConfig.go`（别名转发与策略绑定） | `go test -count=1 -run 'EngineConfig' ./element/time/` |
| MCP 配置写入语义 | `mcp/tools_modify.go`（`handleSetEngineConfig`） | `go test -count=1 -run 'TestToolSetEngineConfig' ./mcp/` |

---

## 1. 类型能力矩阵

### 1.1 现状（为什么需要这张表）

`maths.Number`（`maths/face.go`）声明 `~float32 | ~float64 | ~complex64 | ~complex128`，
但这只是**类型约束**，不是**实现承诺**：引擎里真正被实例化的只有

- `NewMnaUpdateT[float64]` / `NewMnaUpdateSparseT[float64]`（`mna/mna.go`、`load/load.go`、`element/time/`）；
- `NewMnaT[complex128]`（`mna/complex.go` 的 `NewMnaComplex`，唯一消费者 `analysis/ac.go`）。

`float32` 与 `complex64` 在引擎路径**零使用**，且此前**没有任何报错路径**——
泛型约束允许它们编译通过，缺口只会在运行时以错误结果或奇异矩阵暴露。
`mna/typecapability.go` 把这件事变成可判定的契约。

### 1.2 矩阵（路径 × 类型）

求解路径（`mna.SolvePath`）：

| 路径常量 | 取值 | 说明 | 唯一消费者 |
|---|---|---|---|
| `PathTransientDense` | `transient-dense` | 瞬态引擎稠密 LU（float64） | `element/time/simulation.go` |
| `PathTransientSparse` | `transient-sparse` | 瞬态引擎 CSR 稀疏存储 + 稀疏 LU（float64） | `element/time/simulation.go` |
| `PathACPhasor` | `ac-phasor` | 交流相量稳态（complex128） | `analysis/ac.go` |

数值类型（`mna.NumericKind`）：`float32`、`float64`、`complex64`、`complex128`。

| 路径 \ 类型 | float32 | float64 | complex64 | complex128 |
|---|---|---|---|---|
| `transient-dense` | ✗ 不支持 | ✓ 支持 | ✗ 不支持 | ✗ 不支持 |
| `transient-sparse` | ✗ 不支持 | ✓ 支持 | ✗ 不支持 | ✗ 不支持 |
| `ac-phasor` | ✗ 不支持 | ✗ 不支持 | ✗ 不支持 | ✓ 支持 |

不支持的组合**必须返回具体错误值**，不允许静默降级、不允许 panic：

```go
err := mna.CheckNumericType[complex64](mna.PathTransientDense)
errors.Is(err, mna.ErrUnsupportedNumericType) // true；错误文案含路径与类型标识

solver, err := mna.NewMnaUpdateFor[float64](mna.PathTransientSparse, nodes, vsNum)
// 不支持时返回 (nil, 具体错误)；绝不可返回半成品实例
```

查询接口：`mna.TypeCapabilityOf(path, kind)`（单点）、`mna.TypeCapabilityMatrix()`（全量，
与上表逐条对应）、`mna.NumericKindOf[T]()`（泛型参数 → 类型标识）。

### 1.3 新增数值类型时的接入契约

1. 在 `mna` 的 `allNumericKinds` 登记类型标识，并在 `TypeCapabilityOf` 给出每条路径的依据
   （支持/不支持都要写 `Reason`，**不允许默认支持**）；
2. 若某路径要"变为支持"：必须有该类型的 LU 路径 + 至少一条端到端用例 +
   能力矩阵与本文同步更新（三者缺一不算支持）；
3. 未登记的路径或类型一律 `Supported=false`，`CheckNumericType` 报
   `ErrUnknownSolvePath` / `ErrUnknownNumericKind`。

---

## 2. 开关绑定时刻表

`CIRCUIT_*` 环境变量由 `load/load.go` 的 `loadEngineConfig` 解析进 `element.EngineConfig`
（唯一数据源，引擎内部不许 `os.Getenv`）。**语义只增不改**：已有开关的含义不因新增能力而变。

| 开关 | 环境变量 | 绑定时刻 | 运行期可否改 | 生效点 | 校验 |
|---|---|---|---|---|---|
| `EngineConfig.SparseLU` | `CIRCUIT_LUSPARSE` | **载入期绑定** | 改写会改变已绑定后端 → 报错 | `load.LoadContext` 存储选择；`chooseLUSolver` | `element.BackendBinding.Validate` |
| `EngineConfig.ForceDense` | `CIRCUIT_LUSDENSE` | **载入期绑定** | 同上 | 同上 | 同上 |
| `EngineConfig.GminCont` | `CIRCUIT_GMCONT` | 运行期·每次仿真实读 | 可改，下次仿真生效 | `StrategyBindingOf` →（m5）`ApplyStrategy` | — |
| `EngineConfig.NaturalGmin` | `CIRCUIT_NATGMIN` | 运行期·每次仿真实读 | 可改 | 同上 | 负值由 MCP 拒绝 |
| `EngineConfig.GminFinal` | `CIRCUIT_GMINFINAL` | 运行期·每次仿真实读 | 可改 | 同上 | `<=0` 语义 = 回默认值（`ApplyStrategy` 归一） |
| `EngineConfig.GlobalGmin` | `CIRCUIT_GLOBALGMIN` | 运行期·每次仿真实读 | 可改 | 同上 | 负值由 MCP 拒绝 |
| `EngineConfig.GminDbg` | `CIRCUIT_GMIN_DBG` | 运行期·每次仿真实读 | 可改 | 同上 | — |
| `EngineConfig.NoEq` | `CIRCUIT_NOEQ` | 运行期·每次仿真实读 | 可改 | `equilibrateDecompose` | — |

### 2.1 后端绑定（载入期，不可变）

- **绑定判据**：矩阵存储 = `SparseLU || !ForceDense`（稀疏存储），由
  `load.LoadContext` 经 `con.Backend.Bind(sparseLU, forceDense)` 一次性记录；
  存储选择直接取该记录，保证"绑定"与"实际存储"同一判据。
- **互斥**：`SparseLU && ForceDense` 同时为真时 `Validate` / `ResolveBackend` 报错
  （此前只有注释声称互斥，实际靠 `switch` 顺序隐含裁决）。
- **运行期改写**：
  - 载入期由 `CIRCUIT_LUSPARSE` / `CIRCUIT_LUSDENSE` **显式绑定** → 任何后端开关改动都报错；
  - 载入期未显式绑定（自动模式）→ 只拒绝**会改变已绑定存储后端**的写入
    （默认载入即稀疏存储：`forceDense=true` 报错，`sparseLU=true` 是无操作、按原语义放行）。
  - 错误文案必须含 **“载入期绑定”** 四字（机检依赖），且**原值不变**（禁止部分生效）。
- **实际生效后端**：`BackendBinding.ResolveBackend(density, workers) (sparse bool, err error)`，
  与 `element/time/simulation.go` 的 `chooseLUSolver` 逐分支一致：
  `SparseLU` → 稀疏；`ForceDense` → 稠密；`workers > 1` → 并行稠密；
  否则按矩阵非零密度与阈值 `autoSparseDensity = 0.3` 自动选择。非法密度（NaN/负）报错。
- 环境变量同时设置 `CIRCUIT_LUSPARSE` 与 `CIRCUIT_LUSDENSE` 属既存组合，
  **载入期不拒绝**（登记为待清理项），报错发生在求解期（`chooseLUSolver`）与显式校验路径。

### 2.2 策略绑定（运行期，每次仿真实读）

- 视图：`element/time/engineConfig.go` 的 `StrategyBinding` + `StrategyBindingOf(con)`；
- 写入：`func ApplyStrategy(tm *TimeMNA, s StrategyBinding) error`（**由 m5 在
  `element/time/strategy.go` 声明并实现**；m6 只冻结签名与语义，见该文件注释块）；
- 语义：每次仿真启动调用一次、**无条件写** `TimeMNA`；`GminFinal <= 0` = 回默认值；
  遇 NaN/负值报错且不改动 `TimeMNA` 现有状态；不得引入跨 run 残留。

---

## 3. 扩展点接入契约

### 3.1 新元件

1. 实现 `element.ElementFaceList`（`element/face.go`：`ConfigFace` + `ElementFace`）；
2. 在所属域的 `init()` 中调用 `element.AddElement(nodeType, face)`（冲突即 panic，
   见 `element/face.go` 的冲突检测），并确保该域包被 `element/register` 导入；
3. 元件只经 `Stamp` 系列方法写入 MNA 矩阵/右端项，**不得**直接操作 `Context` 内部字段、
   不得读 `os.Getenv`（配置一律经 `element.EngineConfig`）；
4. 储能元件（电容/电感）必须置 `FlagReactive`（`element/mark.go`），否则步长自适应与
   状态导数路径会静默绕过它；
5. 交付判据：网表用例（`cmd/circuits/`）+ 至少一条端到端回归；
   非线性元件必须用**有界**迭代（不得新增"强制推进/静默接受不收敛解"路径）。

### 3.1.1 迭代相关计数：三个口径不得混用（缺陷 D18 的教训）

元件在 `DoStep` 里读迭代数时必须区分这三个语义不同的计数，混用会产生**随时间漂移**的模型：

| 出口 | 语义 | 归零时刻 | 允许用途 |
|---|---|---|---|
| `mna.Time.GoodIterations()` | **累计成功时间步数**（仿真内从不重置） | 从不 | 仅用于"已推进多少步"类判断 |
| `mna.Time.CurrentElemIter()` | **当前时间步**内的元件次级迭代数 | 每轮牛顿外循环开始 | 收敛困难判断（如 PN 结加性 gmin） |
| `mna.Time.CurrentStepNonlinearIterations()` | **当前时间步**内的牛顿外轮数 | 每个时间步开始 | 诊断/统计（CLI `仿真统计:` 行） |

历史缺陷（D18）：`element/semiconductor` 的 `Diode`/`led`/`zener` 曾用 `GoodIterations()` 当
"本步子迭代数"喂给 `subIterations > 100` 的加性 gmin 启发式 → 长仿真（≥500 步）后该条件恒成立、
`extraGmin` 恒被钳到 `1e-6`，**每个 PN 结长期并联 ~1MΩ，模型随仿真时长漂移**。
修正出口为 `CurrentElemIter()`（实现见 `element/time/time.go`，语义见
`element/time/strategy.go` 的 `CurrentStepElemIterations`），器件侧统一改读它；
算式抽到 `element/semiconductor/limiting.go` 的 `extraGminForIterations`，并以
`ExtraGminActivations()` 作为可观测计数。

判据要求（可证伪）：器件侧改动必须有一条**判别式用例**证明"累计步数不影响加性 gmin"
（参考 `limiting_test.go` 的 `TestD18CumulativeStepsMustNotDriveExtraGmin`）；仅有
端到端快照逐位一致**不算**证据——该缺陷在短语料上本就 bit-exact。

### 3.1.2 稀疏 LU 的静态重排判据：按填充数预测择优，不用度数启发式（UD8）

`maths/lu.go` 的 `computeStaticPerm` 在 `n ≥ 500` 且**结构签名变化**时，按
`SymbolicFillCount` 的预测值在「自然顺序」与「最小度重排」之间取填充更少者；
打平保持自然顺序。成本护栏：自然顺序预测填充 `≤ 3n` 时直接保持自然顺序
（带状/三对角的下界本就在 ~2n 量级，最小度不可能显著更优），从而不为常见带状电路
多付最小度重排与第二次预测的开销。

**为什么不用度数比**：旧判据 `maxDeg > 10*avgDeg` 会漏掉两类真实结构——
度数比 ≈4 的 hub（电源/时钟总线连大量节点，但每行度相近）与度数比 ≈1.0 的随机稀疏。
实测（五种 ≥500 合成结构，`maths/orderingCriterion_test.go` 锁定）：

| 结构 | 度数比 | 旧判据 | 新判据 | U nnz（自然 → 最小度） |
|---|---|---|---|---|
| hub600 | 4.02 | 自然（漏检） | 最小度 | 31778 → **4603**（−85.5%） |
| hub800 | 5.24 | 自然（漏检） | 最小度 | 51120 → **5972**（−88.3%） |
| random600 | 1.01 | 自然（漏检） | 最小度 | 107486 → **85316**（−20.6%） |
| band600 | 1.01 | 自然 | 自然（打平） | 4179 → 4179 |
| tridiag1000 | 1.00 | 自然 | 自然（打平） | 1999 → 1999 |

**数值安全**：换消元顺序只引入浮点噪声——`TestPermChoiceNumericalSafety` 断言两种重排下
`A·x=b` 解的最大相对误差 ≤ 1e-6（实测 hub600 2.78e-16 / hub800 7.77e-16 / random600 1.67e-13）。

**验证纪律（重要）**：这是**策略类**改动（允许改变数值结果，须给前后对比 + 回归）。
`41_ram16x8`（n=1690）受其影响：受控交错 A/B（两个二进制、同窗口各 3 次取中位）
实测 `E2EStepRam16x8` **−39.2%**、`E2ELoadRam16x8` −7%（无回归）、Diode 两例持平。
注意：**跨窗口的基线对比在本机负载波动下会给出假回归**（同一改动曾被工具报 `LoadDiode +36%`，
而交错 A/B 显示无变化）——策略类改动应以同窗口交错 A/B 为主证据，基线对比仅作参考。

### 3.1.3 仿真入口的类型契约：`EngineTime`（UD7）

`TransientSimulation(con, cb)` 在本层需要访问**引擎状态**：策略同步、有界牛顿事件计数、
强制重盖标志（`forceStamp`）、gmin 策略读值、连续模式。这些方法此前用
`con.Time.(*TimeMNA)` **逐处断言**，一旦 `con.Time` 被装饰器包造就**静默跳过**
（策略不生效、重盖请求丢失、计数不增），而仿真仍跑完、结果看似正常。

现在入口一次性断言 `element/time.EngineTime` 并**显式报错**：

```go
type EngineTime interface {
    mna.Time
    ApplyStrategyBinding(StrategyBinding) error
    BeginBoundedNewtonEvents()
    ConsumeForceStamp() bool
    AddNewtonIters(n int)
    AddStepHalved(); AddStepRejected(); AddIterExhausted(); AddNaNRejected()
    GminCont() bool; NaturalGmin() float64; GlobalGmin() float64
    SetContinuousMode()
}
```

**为什么声明在 `element/time` 而不是 `mna`**：`ApplyStrategyBinding` 的参数是
`element/time.StrategyBinding`，而 `mna` 不能反向依赖 `element/time`（会成环）。

**装饰器约定（要观测/拦截引擎时照此写）**：

```go
type myObserver struct{ etime.EngineTime }   // 匿名嵌入 → 引擎状态方法整体转发
func (o *myObserver) NextNonlinearIter() bool { /* 统计 … */; return o.EngineTime.NextNonlinearIter() }
```

只嵌入 `mna.Time` 的包装会被入口拒绝，错误信息会点明缺 `EngineTime` 与修法
（宁可报错，也不要静默丢语义）。判据见 `element/time/timeMnaGuard_test.go`：
①只转发 `mna.Time` 的装饰器必须被拒绝；②嵌入 `EngineTime` 的装饰器必须被接受并跑完。

**破坏性但正确**：自行实现 `mna.Time` 的第三方类型若被直接交给 `TransientSimulation`
会开始报错（此前是静默降级）——这是有意的，属 UD7 的修复目标。

### 3.2 新物理域

1. 新建 `element/<domain>/` 包并在 `element/register/register.go` 的导入表中登记；
2. 域内状态必须映射到统一 MNA 方程（`Stamp` 接口），不得旁路 `element.Context`；
3. 引脚类型在 `element/face.go` 的引脚枚举中显式登记（不可复用其它域的引脚编号）；
4. 交付判据：该域的端到端网表 + 退出码 0 的仿真用例 + 与既有域混合的用例
   （参考 `cmd/circuits/11_mixed_domain.net`）。

### 3.3 新求解后端（线性求解器/求解路径）

1. 在 `mna/typecapability.go` 登记路径常量与全量能力条目（含 `Reason`），
   并保证不支持的组合返回 `ErrUnsupportedNumericType`/`ErrUnknownSolvePath`；
2. 后端选择必须在 `element.BackendBinding` 上留痕：载入期能决定存储的，走
   `load.LoadContext` 的绑定；只能运行期决定的，必须经 `ResolveBackend` 分支并写清
   `density/workers` 的判据；
3. `element/time/engineConfig.go`（策略/绑定视图）与本文的绑定时刻表同步更新；
4. 禁止静默降级：后端不可用时返回错误，不得回退到另一后端后只打日志；
5. 交付判据：类型能力矩阵用例 + 后端绑定用例 + 端到端语料逐位一致（回归快照）。

---

## 4. 验收命令（复制即用）

```bash
export GOCACHE=$PWD/.cache/gocache GOPATH=$PWD/.cache/gopath GOMODCACHE=$PWD/.cache/go-mod
go build ./... && go vet ./mna/... ./element/time/... ./load/... ./mcp/...
go test -count=1 ./mna/ ./element/time/ ./mcp/ ./load/
go test -count=1 -run 'TypeCapability|EngineConfig|TestToolSetEngineConfig' ./mna/ ./element/time/ ./mcp/
bash ~/.dsh/skills/code-modularity/scripts/lintCodeModularity.sh \
  element/time/time.go element/time/engineConfig.go element/time/engineConfig_test.go \
  mna/typecapability.go mna/typecapability_test.go --naming --comments
# D18 迭代计数口径（器件侧改动时必须跑）：
go test -count=1 -run 'TestD18' ./element/semiconductor/
```
