# 系统架构

## 1. 数理基础

### 1.1 改进节点分析 (Modified Nodal Analysis, MNA)

MNA 是电路仿真中广泛使用的矩阵构建方法。对于包含 $n$ 个节点和 $m$ 个电压源的电路，构建 $(n+m) \times (n+m)$ 的增广线性方程组：

$$
\begin{bmatrix} \mathbf{Y} & \mathbf{B} \\ \mathbf{C} & \mathbf{D} \end{bmatrix}
\begin{bmatrix} \mathbf{v} \\ \mathbf{i} \end{bmatrix}
=
\begin{bmatrix} \mathbf{j} \\ \mathbf{e} \end{bmatrix}
$$

记为 $\mathbf{A}\mathbf{x} = \mathbf{z}$，其中：

| 子块 | 维度 | 含义 |
|------|------|------|
| $\mathbf{Y}$ | $n \times n$ | 节点导纳矩阵（电阻/电导/电容/电感伴随模型贡献） |
| $\mathbf{B}$ | $n \times m$ | 电压源连接矩阵，$\pm 1$ 表示方向 |
| $\mathbf{C}$ | $m \times n$ | $\mathbf{B}^\mathsf{T}$，电压源约束方程 |
| $\mathbf{D}$ | $m \times m$ | 零矩阵（理想电压源）或受控源跨导 |
| $\mathbf{v}$ | $n$ | 节点电压向量（未知） |
| $\mathbf{i}$ | $m$ | 电压源支路电流（未知） |
| $\mathbf{j}$ | $n$ | 独立电流源向量（已知） |
| $\mathbf{e}$ | $m$ | 独立电压源值（已知） |

**伴随模型**：储能元件（电容/电感）通过数值积分离散化为时域伴随模型。

- **电容伴随模型**（梯形积分）：

$$
i_c^{n} = \frac{2C}{h} v_c^{n} + \underbrace{\left(-\frac{2C}{h} v_c^{n-1} - i_c^{n-1}\right)}_{\text{等效电流源}}
$$

- **电感伴随模型**（梯形积分/后向欧拉）：

$$
\text{梯形: } i_l^{n} = \frac{h}{2L} v_l^{n} + \underbrace{\left(i_l^{n-1} + \frac{h}{2L} v_l^{n-1}\right)}_{\text{等效电流源}}
\qquad
\text{后向欧拉: } i_l^{n} = \frac{h}{L} v_l^{n} + i_l^{n-1}
$$

第一步使用后向欧拉法以保证数值稳定性，后续步使用梯形积分法以获得二阶精度。

### 1.2 Newton-Raphson 迭代

对于非线性元件（二极管、三极管），其伴随模型 $f(\mathbf{x}^{(k)})$ 依赖于解向量本身，需要迭代求解。每轮迭代对非线性函数做一阶 Taylor 展开：

$$
f(\mathbf{x}^{(k+1)}) \approx f(\mathbf{x}^{(k)}) + \mathbf{J}(\mathbf{x}^{(k)}) \cdot (\mathbf{x}^{(k+1)} - \mathbf{x}^{(k)}) = 0
$$

其中 $\mathbf{J} = \partial f / \partial \mathbf{x}$ 为 Jacobian 矩阵。每次对角元作线性化盖章后求解 $\mathbf{A}\mathbf{x} = \mathbf{z}$：

1. **DoStep** 阶段：各元件计算非线性贡献并盖章。
2. **LU 分解**：对更新后的 $\mathbf{A}$ 做 $LU$ 分解。
3. **前代/回代**：求解 $\mathbf{x}^{(k+1)}$。
4. **收敛检查**：计算残差 $\|\mathbf{A}\mathbf{x} - \mathbf{z}\|_2 \leq \varepsilon_{\text{abs}} + \varepsilon_{\text{rel}}\|\mathbf{x}\|_2$。
5. 若不收敛且未达到最大迭代次数，进入元件次级迭代循环。

**收敛条件**：最小 2 轮 Newton 迭代防止线性精确求解导致假收敛；元件次级迭代用于处理交叉耦合元件。

### 1.3 Adams-Bashforth-Moulton 预测-校正法（3 阶）

对于纯 DC 电路（无储能元件），使用 3 阶 Adams 多步法提供 Newton 迭代的初始猜测值并估计局部截断误差 (LTE) 以自适应调整步长。

**预测器 (Adams-Bashforth, 3 阶)**：

$$
\mathbf{x}_{\text{pred}}^{(n+1)} = \mathbf{x}^{(n)} + \frac{h}{12}(23\mathbf{f}^{(n)} - 16\mathbf{f}^{(n-1)} + 5\mathbf{f}^{(n-2)})
$$

**校正器 (Adams-Moulton, 3 阶)**：

$$
\mathbf{x}_{\text{corr}}^{(n+1)} = \mathbf{x}^{(n)} + \frac{h}{12}(5\mathbf{f}_{\text{pred}}^{(n+1)} + 8\mathbf{f}^{(n)} - \mathbf{f}^{(n-1)})
$$

**局部截断误差 (LTE) 估计**：

LTE 为预测器与校正器的差值比例缩放：

$$
\text{LTE} = \left| \frac{C}{C^* - C} \cdot (\mathbf{x}_{\text{corr}} - \mathbf{x}_{\text{pred}}) \right|
$$

其中 $C^* = 3/8$（预测器误差系数），$C = -1/24$（校正器误差系数）。取所有状态分量的最大 LTE 以保证最严误差控制。

**步长自适应**：

$$
h_{\text{new}} = h_{\text{old}} \cdot \left( \frac{s}{\text{LTE} / \varepsilon_{\text{allow}}} \right)^{\frac{1}{p+1}}
$$

- $s = 0.85$：安全系数
- $p = 3$：积分阶数
- $\varepsilon_{\text{allow}} = \varepsilon_{\text{abs}} + \varepsilon_{\text{rel}} \cdot \max|x|$
- 步长变化限制在 $[0.4, 2.5]$ 倍
- 全局步长范围 $[10^{-9}, 5 \times 10^{-4}]$

### 1.4 Doolittle LU 分解

求解 $\mathbf{A}\mathbf{x} = \mathbf{z}$ 的核心线性代数运算。将方阵 $\mathbf{A}$ 分解为下三角阵 $\mathbf{L}$ 与上三角阵 $\mathbf{U}$：

$$
\mathbf{A} = \mathbf{L}\mathbf{U}, \quad l_{ii} = 1
$$

**部分主元法 (Partial Pivoting)**：每列选择绝对值最大的行作为主元并交换，避免除零和提高数值稳定性。

**求解流程**：
1. 前代 (Forward Substitution)：$\mathbf{L}\mathbf{y} = \mathbf{P}\mathbf{z}$
2. 回代 (Backward Substitution)：$\mathbf{U}\mathbf{x} = \mathbf{y}$

支持泛型类型 `float32`、`float64`、`complex64`、`complex128`。并行版本使用递归分块 LU 分解。

---

## 2. 包依赖关系

```
网表文本
    │
    ▼
load/ast (递归下降解析器 + 流式扫描)
    │
    ▼
load/load.go (元件创建 + 子电路展开 + 节点重编号)
    │
    ▼
element/context.go (Context: 运行时上下文 + 元件列表)
    │
    ├──► mna/ (MNA 矩阵盖章接口)
    │       │
    │       ▼
    │   maths/ (泛型线性代数: 矩阵/向量/LU分解)
    │
    └──► element/time/ (瞬态仿真主循环)
            ├── time.go      — TimeMNA: 时间步进 + 预测校正 + 残差 + LTE + 自适应步长
            └── simulation.go — TransientSimulation: Newton-Raphson 迭代 + 收敛控制

稳态分析(独立于 element 时域体系,只依赖 load/ast + maths + mna):
analysis/ (AC 相量 + 小信号混合分析)
  ├── parse.go      — 轻量网表子集解析 (R/C/L/V/I/D/U, 复用 load/ast)
  ├── ac.go         — AnalyzeAC/SweepAC: 复数 MNA 一次求解 + 频率扫描
  ├── linearize.go  — DC 工作点求解 + 二极管工作点线性化 g_d
  ├── small_signal.go — AnalyzeSmallSignal: 同网表双分析(工作点 + 小信号 AC)
  └── powerflow/    — 潮流计算
      ├── bus.go    — Slack/PV/PQ 母线 + 支路模型 + 输入输出结构
      ├── ybus.go   — 复数 Ybus 构建 + 注入功率/失配量
      ├── newton.go — 极坐标牛顿-拉夫逊 (H/N/J/L 雅可比, ΔV/V 形式)
      └── netlist.go — B/BR 潮流网表解析(CLI 批处理入口, 语义与 MCP circuit_run_powerflow 一致)
```

---

## 3. 核心模块

### 3.1 `maths/` — 泛型线性代数

底层数学库，使用 Go 泛型约束 `Number`。

| 类型 | 说明 |
|------|------|
| `Vector[T]` | 稠密向量（接口，支持 Get/Set/Increment/Copy/Zero） |
| `Matrix[T]` | 稠密矩阵 / CSR 稀疏矩阵 |
| `LU[T]` | Doolittle LU 分解 + 部分主元 |
| `luBlock[T]` | 递归分块 LU 分解（无主元） |
| `ParallelLU[T]` | 并行 LU 分解（多 worker 分块） |
| `MatrixVectorMultiply` | 矩阵-向量乘法 |
| `MatrixPruner` | 零行列消除器 |
| `UpdateVector[T]` | 带位图缓存回滚的增量向量 |
| `UpdateMatrix[T]` | 带位图缓存回滚的增量矩阵（16 元素块） |

### 3.2 `mna/` — 改进节点分析接口

| 接口/类型 | 说明 |
|-----------|------|
| `Mna` | MNA 求解器接口：GetA/GetZ/GetX/GetNodeVoltage + 14 种盖章方法 |
| `Time` | 仿真时间接口（见 §3.4） |
| `Trigger` | 触发点：`{Time float64, Triggered bool}` |
| `DerivativeFunc` | 导数函数类型：`func([]float64) ([]float64, error)` |
| `StampCollector` | 盖章操作记录器：14 种操作类型的记录 + 延迟刷新（Flush） |
| `StampCache` | 盖章缓存：基于电压变化检测的增量重算优化 |

### 3.3 `element/` — 元件系统

#### 3.3.1 元件特性位标记 (Flags)

元件通过位掩码标记其特性，统一存储在 `Config.Flags` 字段中（类型 `Flag uint8`）：

```
bit 0: (保留)
bit 1: FlagReactive   (0x02) — 储能元件（C/L），影响步长自适应策略
bit 2: FlagNonlinear  (0x04) — 非线性元件（D/Q），需 Newton-Raphson 迭代
bit 3: FlagCacheStamp (0x08) — DoStep 计算结果仅依赖引脚电压，可启用缓存优化
```

检测方式（通过 `ConfigFace` 接口）：

```go
config.IsFlag(FlagReactive)   // 是否储能元件
config.IsFlag(FlagNonlinear)  // 是否非线性元件
config.IsFlag(FlagCacheStamp) // 是否支持盖章缓存
```

#### 3.3.2 核心结构体

| 结构体/接口 | 说明 |
|-------------|------|
| `Config` | 元件静态配置：名称、引脚定义、参数初始值、Flags |
| `Node` | 元件运行时数据：引脚连接、参数当前值、历史备份（支持 Update/Rollback） |
| `Context` | 仿真上下文：元件列表、MNA 更新器、节点映射、并行选项 |
| `ParallelOptions` | 并行配置：`StampWorkers`（工作线程数）、`CacheThreshold`（缓存阈值） |

#### 3.3.3 核心接口

**`ConfigFace`** — 元件配置接口（所有元件类型必须实现）：

| 方法 | 说明 |
|------|------|
| `Base(ast.ElementNode) *Config` | 初始化配置信息 |
| `InternalNum() int` | 内部节点数量 |
| `GetName() string` | 元件名称 |
| `PinNum() int` | 外部引脚数量 |
| `ValueNum() int` | 参数数量 |
| `VoltageNum() int` | 电压源数量 |
| `Reset(base NodeFace)` | 重置元件状态到初始值 |
| `IsFlag(flag Flag) bool` | 检查 Flags 中是否设置了指定特性位（**统一位检测入口**） |

**`ElementFace`** — 元件行为接口：

| 方法 | 说明 |
|------|------|
| `StartIteration(...)` | 步长迭代开始回调 |
| `Stamp(...)` | 加盖线性贡献到 MNA 矩阵 |
| `DoStep(...)` | 执行非线性元件计算（Newton-Raphson 线性化） |
| `CalculateCurrent(...)` | 计算元件支路电流 |
| `StepFinished(...)` | 步长迭代结束回调 |
| `AddDerivative(...)` | 向导数向量累加状态变量时间导数（仅 FlagReactive 元件） |

#### 3.3.4 Mark 事件枚举

元件回调通过 `Mark` 事件区分执行阶段：

| Mark | 执行时机 |
|------|---------|
| `MarkReset` | 仿真开始，所有元件状态初始化 |
| `MarkStartIteration` | 每个时间步开始，准备新一轮迭代 |
| `MarkStamp` | 加盖线性元件贡献（电阻/电压源等） |
| `MarkDoStep` | 执行非线性计算（二极管的 Newton 线性化等） |
| `MarkCalculateCurrent` | 收敛后计算各支路电流 |
| `MarkStepFinished` | 时间步完成，更新内部状态 |
| `MarkUpdateElements` | 接受当前解，Update（备份当前值） |
| `MarkRollbackElements` | 回滚到上一收敛解 |

### 3.4 `element/time/` — 瞬态仿真

#### 3.4.1 `mna.Time` 接口

`TimeMNA` 实现的完整接口，分为 8 个功能组：

| 组 | 方法 | 说明 |
|----|------|------|
| **参数调整** | `SetTolerances`, `SetStepLimits`, `SetIterationLimits`, `SetMaxTimeSteps` | 配置误差容差、步长范围、迭代上限 |
| **状态查询** | `CurrentTime`, `CurrentStep`, `TargetTime`, `GoodIterations`, `ResidualNorm` | 查询当前仿真状态 |
| **预测-校正** | `Predict`, `Correct`, `CopyPredStateToX` | Adams-Bashforth 预测 + Adams-Moulton 校正 |
| **历史管理** | `InitHistory`, `BootstrapHistory`, `UpdateHistory`, `SetCorrStateFromX` | 多变量历史数据初始化与推进 |
| **残差计算** | `CalculateMNAResidual`, `CheckResidualConvergence` | MNA 残差范数计算与收敛判定 |
| **LTE & 步长** | `EstimateLTE`, `AdjustStepSize` | 局部截断误差估计与自适应步长 |
| **迭代控制** | `ResetNonlinearIter`, `NextNonlinearIter`, `ResetElemIter`, `NextElemIter` | 非线性/元件迭代计数管理 |
| **时间推进** | `AdvanceTimeStep`, `AdvanceTimeSimple` | 时间步推进（含触发点截断和目标时间截断） |

#### 3.4.2 两种历史初始化路径

| 方法 | 适用场景 | 实现 |
|------|---------|------|
| `InitHistory` | 含储能元件的电路 | 用改进欧拉法一步生成 3 步历史 |
| `BootstrapHistory` | 纯 DC 电路 | 逐步累积前 3 步 Newton 收敛解 |

#### 3.4.3 数值参数默认值

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `absTol` | $10^{-6}$ | 绝对误差容差 |
| `relTol` | $10^{-4}$ | 相对误差容差 |
| `safety` | $0.85$ | 步长调整安全系数 |
| `maxNonlinIter` | $200$ | 最大 Newton 迭代次数 |
| `maxElemIter` | $100$ | 最大元件次级迭代次数 |
| `maxTimeSteps` | $10000$ | 单步内最大尝试次数（防死循环） |
| `currentStep` | $10^{-6}$ | 初始步长 |
| `minStep` | $10^{-9}$ | 最小允许步长 |
| `maxStep` | $5 \times 10^{-4}$ | 最大允许步长 |

### 3.5 `load/` — 网表加载

#### 子电路支持

支持 `.subckt` / `.ends` 块定义子电路（可嵌套），加载器递归展开为实际元件并自动分配内部节点 ID。

```
.subckt nand a b out
  R1 [100,out] [1000]
  Q1 [a,1,0] [false,100]
  Q2 [b,1,0] [false,100]
.ends nand

X1 [in1,in2,out] [nand]   # 实例化 nand 子电路
```

#### 节点重编号

用户网表中的节点 ID 可为任意非连续整数（如 10, 11, 20, 100, -1），MNA 矩阵需紧凑连续索引（0, 1, ..., N-1）。加载流程：

1. **第一遍扫描**：解析网表、展开子电路、创建元件、收集所有引脚节点 ID
2. **去重排序**：正节点 ID 排序后映射到 `compactID`（0, 1, 2, ...）
3. **重写引脚**：将所有元件引脚节点替换为 `compactID`
4. **保存映射**：`Context.CompactNodeID` 供用户代码查询

### 3.6 `gpio/` — 通用 IO 控制

提供仿真器与外部世界交互的接口：引脚读写、位操作、寄存器访问。用于连接仿真电路到虚拟外设。

---

## 4. 仿真数据流

```
LoadString(netlist) → Context
    ↓
Context.Time = NewTimeMNA(endTime)     # 创建 Time 实例
    ↓
TransientSimulation(con, callback)
    │
    ├── Mark.Reset → 元件状态初始化
    ├── Mark.Stamp → 构建线性 MNA 矩阵
    ├── Mark.StartIteration → 通知元件新迭代开始
    │
    ├── 时间循环（每个时间步）:
    │   │
    │   ├── [DC电路] Predict → CopyPredStateToX → Newton 初始猜测
    │   │
    │   ├── Newton-Raphson 外循环:
    │   │   ├── Mark.DoStep → 非线性元件线性化盖章
    │   │   ├── LU.Decompose + SolveReuse → 求解 A·x = z
    │   │   ├── CalculateMNAResidual → 计算残差 ||Ax - z||₂
    │   │   ├── CheckResidualConvergence → 收敛判定
    │   │   │
    │   │   └── [不收敛] 元件次级迭代循环:
    │   │       ├── Mark.RollbackElements → 回滚
    │   │       ├── Mark.DoStep → 重新计算
    │   │       └── 重新求解 + 收敛检查
    │   │
    │   ├── Mark.CalculateCurrent → 计算支路电流
    │   ├── [DC电路] EstimateLTE + AdjustStepSize → 步长自适应
    │   ├── BootstrapHistory [前3步] / UpdateHistory [后续] → 历史维护
    │   ├── AdvanceTimeSimple → 推进时间 + 触发点处理
    │   ├── Mark.UpdateElements → 接受解
    │   └── callback(voltages) → 输出结果
    │
    └── 仿真结束
```

---

## 5. 并行仿真

### 5.1 DoStep 并行化

`ParallelCallMark(MarkDoStep)` 将元件列表分片到 `N` 个 worker goroutine：

```go
workers := ParallelOpts.StampWorkers  // 默认 = GOMAXPROCS
```

每个 worker 处理一片元件，独立创建 `StampCollector` 记录盖章操作，完成后按原始顺序 Flush 到全局 MNA 矩阵。

### 5.2 盖章缓存优化

对于 `FlagCacheStamp` 标记的元件（Diode, Transistor），DoStep 计算结果仅依赖引脚电压。若电压未变化则复用缓存结果：

```go
if cache != nil && !cache.NeedsBuild() && !cache.HasChanged(con) {
    // 直接复用缓存的盖章记录，跳过 DoStep 计算
}
```

缓存失效条件：引脚电压变化超过 `CacheThreshold`。

---

## 6. 泛型约束

项目使用 Go 泛型统一支持多种数值类型：

```go
type Number interface {
    float32 | float64 | complex64 | complex128
}
```

| 类型 | 适用场景 |
|------|---------|
| `float64` | 电子电路瞬态仿真 |
| `float32` | 轻量级仿真 / 嵌入式 |
| `complex64/128` | 交流频域分析（AC 相量）、潮流计算（均已落地，见 analysis/powerflow 包） |

复数的线性代数与实数的 LU 分解共享同一泛型实现。MNA 盖章方法（含复数 1/z 分支、
复数 NaN 过滤）全部泛型化；`mna.NewMnaComplex`/`NewMnaUpdateComplex` 提供
complex128 实例化入口（`mna/complex.go`）。

---

## 7. 文件结构

```
circuit/
├── maths/           # 泛型线性代数（矩阵、向量、LU 分解、均衡化）
├── mna/             # MNA 接口 + 复数实例化（complex.go）
├── element/         # 元件系统（按物理域分目录: passive/source/logic/...）
│   ├── config.go / face.go / mark.go / context.go / parallel.go
│   ├── register/        # 统一注册入口（blank import 全部元件）
│   ├── time/            # 瞬态仿真主循环（time.go + simulation.go）
│   └── {passive,source,controlled,semiconductor,logic,switch,...}/
├── analysis/        # 稳态分析: AC 相量（ac.go）+ 小信号（small_signal.go）
│   └── powerflow/       # 潮流计算（bus.go + ybus.go + newton.go）
├── load/            # 网表解析与加载（AST 解析器 + 子电路展开）
├── mcp/             # MCP 服务器（25 工具: 会话/检视/修改/仿真/导出/稳态分析）
├── gpio/            # 通用 IO 接口
├── cmd/             # CLI 入口（circuit 子命令: 批处理仿真/交互 TUI/MCP 服务器/潮流求解）
│   ├── main.go      # 参数解析、模式路由、批处理仿真与 html 输出
│   ├── powerflow.go # B/BR 潮流网表检测(isPowerflowNetlist)与 csv/tsv/table/html 输出
│   ├── tui.go       # bubbletea 交互界面
│   ├── plot.go      # gonum/plot PNG 曲线
│   ├── circuits/    # 29 个测试网表(01~26 时域, 27~29 稳态分析)
│   └── lcd_test/    # LCD 测试程序
├── webout/          # 共享 HTML 波形页模板（cmd 与 mcp 共用）
└── docs/            # 项目文档
```
