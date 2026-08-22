# circuit
Go实现的电气仿真,通过底层泛型与接口统一实现对 电子元件,气路元件,油路元件 综合综合仿真.通过事件同步实现 逻辑电路,潮流计算 的联动仿真——时域(瞬态/逻辑)与稳态(AC 相量/小信号/潮流)双引擎,见 [docs/powerflow.md](docs/powerflow.md).

## MCP 服务器
提供 MCP (Model Context Protocol) 服务器，可通过 LLM 客户端（Claude Desktop / dsh / Cursor）直接进行电路仿真：
网表加载与校验、元件/节点检视、参数修改、事件驱动、异步瞬态/DC 仿真、稳态分析（AC 相量 / 频率扫描 / 潮流计算）、结果获取与 HTML 波形导出。
共 25 个工具（会话 5 + 检视 6 + 修改 4 + 仿真 5 + 导出 2 + 稳态分析 3）。

## 稳态分析（analysis / powerflow 包）
- **AC 相量分析**：复数 MNA 一次求解，节点幅值/相位/复功率，频率扫描（`circuit_run_ac` / `circuit_sweep_ac`）
- **小信号混合分析**：DC 工作点 + 工作点线性化（二极管 g_d、逻辑门输出短路），逻辑电路与模拟网络频域联动
- **潮流计算**：Slack/PV/PQ 母线 + 极坐标牛顿-拉夫逊，母线电压/注入功率/线路潮流/网损（`circuit_run_powerflow`）
- 详见 [docs/powerflow.md](docs/powerflow.md)；验证电路 cmd/circuits/27~29

```bash
# stdio（默认） / HTTP / SSE
go run ./cmd mcpserver
go run ./cmd mcpserver -transport http -addr :18080
```

详细工具列表与使用示例见 [docs/mcp.md](docs/mcp.md)。

## 当前Bug列表
  (暂无已知Bug)


## 开发日志
  * [2025-11-17] 准备重写
    * utils 实现基础底层结构包  
      1. Bitmap 位图标记结构
      2. Flag 事件标记管理
    * maths 矩阵实现
  * [2025-11-18] 实现矩阵相关内容
    1. 基础的矩阵与向量结构体实现
    2. 基础结构的lu分解实现
  * [2025-11-20] 增加 Mna 矩阵
    1. 添加 mna 矩阵加盖实现
    2. 修改 maths 包实现
    3. 增加参考技术文档
    4. 实现基础结构定义
  * [2025-11-21]
    1. 尝试构建基础元件底层实现
  * [2025-11-21]
    1. 尝试基础电路仿真
  * [2025-11-23]
    1. 实现最基础的电子元件仿真
  * [2025-11-25]
    1. 将元件配置独立为 ElementConfigBase 结构体
  * [2025-11-27]
    1. 测试电路的运放仿真
  * [2025-11-28]
    1. 修改部分代码
  * [2025-11-30]
    1. 增加测试元件实现
  * [2025-12-2]
    1. 增加 time 包实现迭代处理
    2. 测试 RC运放跟随 电路通过
  * [2025-12-12]
    1. 实现基础元件
  * [2025-12-15]
    1. 补全基础元件
  * [2025-12-29]
    1. 测试基础元件处理异常bug
  * [2025-12-30]
    1. 将 element 包移动到 utils 包下,并规划新的基于事件传递的并向处理实现
    2. 处理基础元件 二极管,运放 的错误
  * [2025-12-31]
    1. 准备移除现有的事件实现
  * [2026-1-3]
    1. 执行大量修改,优化元件实现与接口定义
  * [2026-1-4]
    1. 增加基础元件的测试文件
    2. 分离 VoltageID 与 NodeID 定义
    3. 修复lu分解在特殊情况下会产生的严重的bug
  * [2026-1-7]
    1. 代码优化
  * [2026-1-8]
    1. 潮汐计算需要底层能够处理基于复数的计算所以对底层进行泛型化处理
    2. 优化接口
  * [2026-1-9]
    1. 实现矩阵优化
  * [2026-1-12]
    1. 代码优化
    2. 将 mna 接口改为泛型
    3. 实现基础的虚拟机vm指令运行环境,使用 rv32imafd 指令集
  * [2026-1-13]
    1. 修复缺失的指令
    2. 拆分指令为列表
  * [2026-1-16]
    1. 尝试实现 Context 结构体
    2. 尝试实现并发处理
  * [2026-1-18]
    1. 完善功能
  * [2026-1-20]
    1. 修改 vm 测试代码为 c 实现
    2. 为基础元件添加变量名称信息
  * [2026-1-21]
    1. 修改网表加载过程使网表支持动态元件
  * [2026-1-26]
    1. 重写网表加载实现
  * [2026-1-27]
    1. 优化网表 ast 解析实现
  * [2026-1-31]
    1. 优化vm虚拟机
    2. 开始编写 app 界面
  * [2026-2-2]
    1. 成功使用vm虚拟机加载linux内核,目前vm虚拟机已可用.
    2. 开始编写 app 基础控件
  * [2026-2-3]
    1. 实现 网格容器 控件的实现
  * [2026-2-5]
    1. 尝试实现 节点编辑器 控件
  * [2026-2-12]
    1. 完善 节点编辑器 控件功能
  * [2026-2-13]
    1. 完成 节点编辑器 基础功能
    2. 确认 节点编辑器 的基础功能可用
  * [2026-4-20]
    1. 移除gui的实现
    2. 增加 gpio 的操作实现
  * [2026-4-24]
    1. 实现并发计算
  * [2026-4-26]
    1. 增加时间事件与补全注释
    2. 实现子电路处理
  * [2026-4-28]
    1. 实现并发计算与子电路处理
  * [2026-5-3]
    2. 缺陷修复
  * [2026-5-27] 实现双缓冲结构体与列感知压缩
    1. 实现 utils/doublebuffer 泛型双缓冲包,支持 [dt+1][x]T 多维数据存储
    2. 两个缓冲区交替工作,dt>=n 自动切换并写入文件
    3. 支持 Zlib/DeltaZlib/Noop 三种压缩器
    4. 实现 BlockWriter/BlockReader 二进制块文件格式 (魔数+dt+x+压缩数据)
    5. 新增 BufferReader[T] 泛型读取器,自动完成 读取→解压→反序列化
    6. 通过代码审查+对抗验证修复 4 个严重缺陷 (类型检测、越界、OOM防护、命名类型安全)
  * [2026-5-27] 实现 RLE 列感知零值跳过压缩
    1. 新增 BlockCodec[T] 接口与 FlatCodec/RLECompressor 两种实现
    2. RLE 按列独立编码: base_value + [ZERO_SKIP(run) | DELTA(delta)]* 格式
    3. 全稳定信号压缩率 0.4% (8000B→33B),RC电路实测压缩率提升 38%
    4. 增加 RLE 解码器越界保护 (Uvarint/slice 边界校验)
  * [2026-5-28] 实现事件系统与交互元件
    1. Context 增加事件系统: sync.Map 存储事件值,SetEvent/PushEvents/PullEvents 方法
    2. Config 增加 EventSlots map[int]int: nameIdx→±valueIdx,正数消费者负数生产者
    3. 每步时序: PushEvents(步头)→仿真计算→PullEvents(步尾,生产者回写)
    4. 新增 EventSwitch(B) 元件: 事件驱动按钮/触点,支持 NO/NC,Base()覆盖实现动态引脚
    5. 新增 EventResistor(VR) 元件: 事件值 0~1 线性映射到 minR~maxR
    6. 新增 Coil(RLY) 元件: 继电器线圈,RL串联梯形积分模型,磁滞(I_pullin/I_hold),多通道事件输出
    7. Node Set 方法增加类型断言,eventTargets 从裸指针改为 NodeFace+index 防悬空
  * [2026-5-28] 创建测试电路与缺陷修复
    1. 新增 cmd/volt_record RC电路电压记录测试 (5000步,双缓冲→RLE压缩→文件→读回验证)
    2. 新增 cmd/relay_test 继电器线圈+触点联动测试 (NO/NC触点,一帧延迟模拟机械动作)
    3. 修复 EventSwitch Stamp/DoStep 阻抗盖章时序 (回滚后阻抗丢失)
    4. 修复 Coil RL 梯形积分 I_hist 计算 (VL vs Vdiff) 与 Norton 等效缩放
    5. 修复 MarkReset 中 EventSlots 负值索引未过滤导致 producer 元件 panic
    6. 删除 Node.EventBinding 冗余中间层,eventTargets 直接存 Context
   * [2026-5-28] 实现CLI命令行仿真主程序
     1. 新增 cmd/main.go CLI入口程序,支持命令行参数解析(flag包)
     2. 支持 --time/-t 仿真时间、--output/-o 输出文件、--step/--min-step/--max-step 步长控制
     3. 支持 --abs-tol/--rel-tol 容差、--max-iter/--max-steps 迭代限制
     4. 支持 --nodes 节点过滤、--parallel 并行worker、--format csv/tsv/table 三种输出格式
     5. 支持 --quiet 静默模式、-h/--help 帮助信息
     6. 核心流程: 加载网表→创建TimeMNA→配置参数→TransientSimulation→格式化输出
     7. 错误处理: 区分性退出码(1-4),errWriter写入错误检测,bufio文件缓冲
     8. 通过代码审查修复: 写入错误静默忽略、defer关闭错误、table OOM保护、并行参数校验
   * [2026-5-28] 创建测试电路文件夹
     1. 新增 cmd/circuits/ 目录,包含9个测试电路网表和README说明文档
     2. 01_resistor_divider.net — 电阻分压器(DC分析验证)
     3. 02_rc_filter.net — RC低通滤波器(瞬态充电曲线)
     4. 03_diode_rectifier.net — 二极管半波整流(非线性元件)
     5. 04_transistor_switch.net — NPN三极管开关(饱和/截止)
     6. 05_opamp_amplifier.net — 运放同相放大器(增益验证)
     7. 06_voltage_sources.net — 多波形演示(DC/正弦/方波/三角波/脉冲)
     8. 07_rlc_circuit.net — RLC串联振荡(电感+电容瞬态)
     9. 08_subcircuit.net — 子电路级联(.subckt/X实例化,3级RC)
    10. 09_half_adder.net — RTL半加器(8-NOR门,16三极管,数字逻辑)
    11. 修复子电路网表解析兼容性(X实例化括号格式、元件ID命名规则)
   * [2026-5-28] 重写gpio/gui/style.go UI样式系统
     1. 从101行半成品(动画逻辑注释)重写为462行完整样式系统
     2. State 位掩码: 7种状态(Hovered/Pressed/Focused/Disabled/Active/Selected),Match/Has/String方法
     3. Value[T] 泛型状态驱动值: NewValue+On链式覆盖+Resolve倒序匹配
     4. Easing 缓动函数库: Linear/InOut(Quad×3/Cubic×3/Quart×3)+Bounce+Elastic 共11种
     5. Interpolator[T] 泛型插值器: LerpColor(RGB565三通道)/LerpFloat32/LerpInt
     6. Animator 动画管理器: AnimateTo/Resolve/IsDone/Cancel/Clear,多属性并行过渡
     7. AnimatedValue[T] 自动过渡: 检测目标值变化→启动动画→缓动插值
     8. ButtonStyle 预设示例: DefaultButtonStyle() 展示状态驱动样式用法
   * [2026-5-29] 实现交互式连续仿真系统
     1. 新增 element/time 连续仿真模式: SimStatus 状态机(Running/Paused/Stopped/Stepping)
     2. TimeMNA 新增 SetContinuousMode/Pause/Resume/Stop/StepOnce/AdvanceFor/Status 方法
     3. 仿真主循环每步检查状态: Paused→自旋等待, Stopped→退出, Stepping→一步后暂停
     4. AdvanceFor(dur) 前进 N 秒后自动暂停, 支持临时目标时间+自动恢复连续模式
     5. 新增 utils/doublebuffer 纯内存模式: flushActive/flushNonActive 无 writer 时不报错
     6. 新增 doublebuffer 环形缓冲: SetMaxBlocks/LastRows/TotalRows/BlockCount 示波器深度
     7. 修复 Append 在 writer 为空时 dt 越界 panic
   * [2026-5-29] 实现 TUI 交互界面 (bubbletea)
     1. 新增 cmd/tui.go (770+行), 基于 charmbracelet/bubbletea+bubbles+lipgloss
     2. 双栏布局: 左侧 viewport(节点电压/输出历史) + 右侧 sidebar(状态/快捷键)
     3. 命令输入: textinput 组件, Tab 补全, ↑↓ 历史浏览
     4. 焦点切换: 空输入框+↑ 切到浏览模式, Esc 切回; 浏览模式中 ↑↓ 滚动 viewport
     5. 鼠标滚轮支持: tea.WithMouseCellMotion() + MouseMsg 转发 viewport
     6. 自适应表格: 帮助和 curve 输出使用动态列宽, 窗口缩放自动适配
     7. 命令分隔: 每次命令输出前加 ··· 分隔线, 帮助用 ┌┬┐ 表格框线
   * [2026-5-29] 交互式仿真功能完善
     1. voltage 触发: trigger <节点> <op> <值> 设置条件, 到达自动暂停
     2. 历史曲线: curve <节点> <秒数> [点数] 从环形缓冲取时间窗口数据
     3. 超缓冲保护: curve 请求超出缓冲区范围时自动截断+警告
     4. 事件设置: set <事件> <值> 实时注入事件到仿真上下文
     5. 状态查询: status 显示运行状态/步数/时间
     6. 命令集: v/t/run/step/curve/trigger/set/pause/resume/stop/status/help 共 12 个
    * [2026-5-30] TUI 交互增强: Plot 绘图、步长与缓冲状态显示
      1. 新增 cmd/plot.go (186行): 基于 gonum/plot v0.17 专业科学绘图库生成 PNG 电压曲线
      2. plot 命令: `plot <节点|all> [文件名]` 直接绘制缓冲区全部历史数据
      3. 新增 cmd/table_util.go (118行): 自适应终端表格渲染，根据内容与终端宽度计算列宽
      4. TUI 标题栏新增当前自适应步长 `dt=%.2e s` (TimeMNA.CurrentStep)
      5. TUI 侧边栏新增缓冲区信息 `缓冲 N行 M/K块` (TotalRows/BlockCount/MaxBlocks)
      6. 新增 go.mod 依赖: gonum.org/v1/plot v0.17.0, 升级 x/image v0.26→v0.30
  * [2026-6-3] 元件目录全面重组：从单目录到14个领域目录
    1. 将 element/base/ 下20个元件按物理领域拆分到13个子目录
    2. 目录结构: passive(R/C/L) source(V/I) controlled(VCVS) semiconductor(D/Q) 
       analog(OpAmp) logic(Gate/DFF/Sync) switch(SW/B/MB) interactive(VR/RLY)
       electromechanical(Motor) magnetic(XFMR) hierarchical(Wrapper)
    3. 新增 element/register/ 统一注册入口，聚合所有子包 blank import
    4. 更新 cmd/main.go 和 load/bus_expand_test.go 导入
    5. 所有现有测试保持通过，零回归
  * [2026-6-3] 实现22个新基础元件
    1. 受控源: VCCS(G)/CCCS(F)/CCVS(H)，MNA接口StampVCCS/CCCS/CCVS已就绪
    2. 气路元件(PinPneumatic): PR气阻/PC气容/PL气感/PCV单向阀/PS气源
       - 基于压力↔电压类比，复用电路元件MNA模型
       - 气容/气感使用梯形积分伴随模型(FlagReactive)
    3. 油路元件(PinHydraulic): HR液阻/HA蓄能器/HL液感/HCV单向阀/HP液压泵
       - 类比气路实现，使用PinHydraulic引脚类型
    4. 半导体扩展: Z稳压管(齐纳封装)/LED(低电流二极管)/MOSFET(LEVEL=1 NMOS/PMOS)/JFET(NJF/PJF)
       - MOSFET使用Shichman-Hodges模型，Newton-Raphson线性化加盖
    5. 逻辑扩展: JK触发器/T触发器/SR锁存器/CMP比较器/ST施密特触发器
       - 边沿触发型参考DFlipFlop模式，组合逻辑型参考Gate模式
  * [2026-6-3] 实现4个跨域传感器元件
    1. 新增 element/sensor/ 目录，混合引脚类型实现跨物理域转换
    2. PSENS压力传感器: 气路/液压压力→电压信号，VCVS模式+高阻抗隔离
       - Base()方法根据domain参数动态切换PinPneumatic/PinHydraulic
    3. EP电气-气动转换器: 电压→气路压力，默认gain=100kPa/V
    4. EH电气-液压转换器: 电压→油路压力，默认gain=1MPa/V
    5. CS电流传感器: 支路电流→电压信号，内部0V测量电压源+CCVS模式
    6. 所有传感器使用手动构建[]Pin数组实现混合引脚类型
  * [2026-6-3] 混合物理域数值稳定性分析与完整修复
    1. 深入分析MNA求解器在电气+气动+液压混合仿真中的数值问题
    2. 发现3个严重问题:
       a) 矩阵无缩放预处理，元素跨14个数量级(1e-9~1e6)，条件数κ≈10^14
       b) 全局L2范数收敛判据被大数值域(气压~1e5Pa)主导，淹没电气域精度
       c) 自适应步长受大域驱动，无混合域验证测试
    3. 修复方案:
       a) 新增 maths/equilibrate.go 行+列均衡化LU分解
          - EquilibrateAndDecompose: A'[i][j]=R[i]·A[i][j]·C[j]，条件数降至≈1
          - SolveEquilibrated: 自动处理b行缩放和x列反缩放
       b) 修改 element/time/simulation.go 两处LU调用使用均衡化
       c) 修改 element/time/time.go 收敛判据: 全局L2→分量级独立检查
          - 每个解分量: |residual_i| ≤ absTol + relTol·max(|X_i|,1.0)
       d) 新增 cmd/circuits/11_mixed_domain.net 混合域测试网表
  * [2026-6-3] 修复MNA受控源符号Bug
    1. 发现 mna/mna.go 中 VCCS(StampVCCS)和CCCS(StampCCCS)符号错误
    2. 受控源LHS矩阵贡献与StampCurrentSource约定不一致，导致输出电流方向错误
    3. 修正两函数共6处符号
  * [2026-6-3] 补充20个测试用例覆盖新增元件
    1. maths/equilibrate_test.go: Hilbert病态矩阵(条件数1.5e7)精度验证
       跨数量级矩阵(1e-9~1e6)均衡化验证、奇异矩阵错误处理
    2. element/controlled/: VCCS/CCCS/CCVS功能测试(3项)
    3. element/sensor/: 压力传感器/EP转换器/电流传感器端到端测试(3项)
    4. element/pneumatic/: 气阻分压/气容RC/单向阀测试(3项)
    5. element/hydraulic/: 液阻分压/蓄能器/EH转换器测试(3项)
    6. element/logic/: 比较器/施密特触发器/SR锁存器测试(3项)
    7. element/semiconductor/: 稳压管钳位/LED正向导通测试(2项)
    8. 测试发现并修正网表语法(GND为-1而非0)、参数顺序等6个问题
  * [2026-6-3] 补全26个源文件Go文档注释
    1. 统一注释规范: 文件头模块说明+变量(类型标识+网表格式)+类型(数学模型)+方法(参数/步骤)
    2. 覆盖: controlled(3) pneumatic(5) hydraulic(5) semiconductor(4) sensor(4) logic(5) maths(1)
  * [2026-8] 实现稳态分析三层能力:AC 相量分析 / 小信号混合分析 / 潮流计算
    1. mna 复数实例化(NewMnaComplex)+ oneOf[T] 修复复数 Stamp 类型断言 panic
    2. analysis 包:AnalyzeAC/SweepAC(复数 MNA 一次求解,RC -3dB 验证)
    3. 小信号:DC 工作点(element 体系)+ 二极管工作点线性化 + 逻辑门输出短路
    4. 修复 element 二极管 Rs=0 内部节点悬空导致开路
    5. powerflow 包:牛顿-拉夫逊(H/N/J/L 雅可比,PV 无功越限转 PQ)
    6. MCP 新增 3 工具(22→25),集成测试全绿
  * [2026-8] MCP 稳态分析工具落地 (工具总数 22→25)
    1. circuit_run_ac: 线性 AC 相量分析(复数 MNA 一次求解,幅值/相位/复功率)
    2. circuit_sweep_ac: 频率扫描(对数/线性,默认对数)
    3. circuit_run_powerflow: 潮流计算(buses/branches 数组参数,标幺值,Theta 度)
    4. 功率/复数统一输出 {p,q}/{mag,phaseDeg,real,imag} 结构
    5. 验证电路 27_ac_filter / 28_small_signal / 29_powerflow_3bus
  * [2026-8] CLI 直接运行 B/BR 潮流网表 (29 号算例可执行)
    1. analysis/powerflow/netlist.go: ParseNetlist 解析 B<id> slack|pv|pq + BR<id> From-To
    2. 键大小写不敏感,错误带行号,语义与 MCP circuit_run_powerflow 一致
    3. cmd/powerflow.go: isPowerflowNetlist 检测(第二词元 slack/pv/pq,防误判 B 开关元件)
    4. 输出 csv/tsv/table/html 四种格式;TUI 模式遇潮流网表明确报错
    5. 实测 29 号 exit 0,01/10 号普通网表回归无误判
  * [2026-8] CLI 与 MCP 服务器支持 pprof 性能剖析
    1. 新增 cmd/pprof.go: --pprof HTTP 调试服务(net/http/pprof) / --cpuprofile CPU 采样 / --memprofile 堆画像
    2. 批处理/interactive 与 mcpserver 子命令均可启用;pprof 在进程退出前完整落盘(main 重构为 run() 保证 defer 可靠执行)
    3. 用法: circuit --pprof :6060 --cpuprofile cpu.prof circuit.net;运行中可 go tool pprof http://:6060/debug/pprof/profile
  * [2026-8] 引擎标准 SPICE Gmin stepping (解决交叉耦合锁存器 t=0 对称振荡不收敛)
    1. 延续法: 从大 Gmin(1e-3 S) 给每个 PN 结并联低阻把双稳态阻尼成单稳态,收敛后逐级 ×0.1 降至 1e-12 逼近真实工作点
    2. 仅 t=0 直流求解步启用,后续步恢复自然 gmin;开关 CIRCUIT_GMCONT=1(默认关闭,不影响普通电路),调试 CIRCUIT_GMIN_DBG=1
    3. element/time/time.go 状态机(Begin/StepGminDown/RecoverGmin/End) + simulation.go 每步调度/残差判据耦合 + PN 结元件 DoStep 读取延续值
  * [2026-8] 稀疏 LU 接入与性能优化 (CIRCUIT_LUSPARSE=1 切换 CSR 稀疏链路)
    1. 修复 luSparse 两处数值 bug: L 主元行交换只交换前 k 列;消元不再用绝对阈值丢弃 fill-in(避免近奇异矩阵误判奇异)
    2. 新增 NewMnaUpdateSparse 工厂 + EquilibrateAndDecomposeSparse 稀疏均衡化,load/simulation 按开关联动
    3. rowSparseMatrix 行切片存储替代 CSR 单数组,消除 fill-in 插入的 O(n^4) 退化(memmove 44.6%→append O(1))
    4. 300 节点 RC 链 ~17.7x 加速;2210 节点局部 RTL 连接分解 78~547ms;稀疏/稠密输出逐位一致

## 开发任务规划
  1. [✔] 实现基于计算图构建矩阵方程求解器  
  2. [✔] 实现 MNA 求解器与基础接口定义  
  3. [✔] 实现 基础元件  
  4. [✔] 实现 加载与导出 
  5. [✔] 规划元件并行计算实现  
  6. [✔] 增加虚拟机,用于加载指令集  
  7. [✔] 实现 MCP 服务器（25 工具 + stdio/HTTP/SSE 传输，见 docs/mcp.md）
  8. [✔] 实现稳态分析三层:AC 相量 / 小信号混合 / 潮流计算（见 docs/powerflow.md）
  9. [✔] CLI 直接运行 B/BR 潮流网表（29 号算例可执行，见 docs/powerflow.md §3）
  10. [✔] 引擎标准 SPICE Gmin stepping（CIRCUIT_GMCONT=1，解决交叉耦合锁存器 t=0 振荡不收敛）
  11. [✔] 稀疏 LU 接入与性能优化（CIRCUIT_LUSPARSE=1，行切片存储消除 fill-in 插入 O(n^4) 退化）
  12. [✔] CLI/MCP 服务器支持 pprof 性能剖析（--pprof / --cpuprofile / --memprofile）


## 实现过程
  ## 基础设计
  > 需要将仿真器与前端隔离并且通过事件进行操作  
  > 也就是说前端通过通道传递事件到后端进行仿真器的控制

  ## TUI 交互
  > 基于 bubbletea 的终端 UI, 仿真在后台 goroutine 持续运行
  > 用户通过命令输入实时查询电压、设置事件、控制仿真
  > 支持 Tab 补全、↑↓ 历史、鼠标滚轮、焦点切换
