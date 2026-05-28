# circuit
Go实现的电气仿真,通过底层泛型与接口统一实现对 电子元件,气路元件,油路元件 综合综合仿真.通过事件同步实现 逻辑电路,潮汐计算 的联动仿真.
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

## 开发任务规划
  1. [✔] 实现基于计算图构建矩阵方程求解器  
  2. [✔] 实现 MNA 求解器与基础接口定义  
  3. [✔] 实现 基础元件  
  4. [✔] 实现 加载与导出 
  5. [✔] 规划元件并行计算实现  
  6. [✔] 增加虚拟机,用于加载指令集  


## 实现过程
  ## 基础设计
  > 需要将仿真器与前端隔离并且通过事件进行操作  
  > 也就是说前端通过通道传递事件到后端进行仿真器的控制

  ## TUI 交互
  > 基于 bubbletea 的终端 UI, 仿真在后台 goroutine 持续运行
  > 用户通过命令输入实时查询电压、设置事件、控制仿真
  > 支持 Tab 补全、↑↓ 历史、鼠标滚轮、焦点切换
