// limiting.go —— 非线性器件的迭代电压限制（HSPICE/Spectre 式 pnjlim）。
//
// 职责：在每次牛顿迭代中，把 PN 结 / 栅结电压相对上一次迭代的变化量限制在
// maxStep 伏以内（默认 0.25V，取值区间 [1e-3, 5]V），抑制指数结在大信号跳变时的
// 过冲，降低未收敛信号与减半步长重试次数。
//
// 开关语义：**默认关闭**。关闭时 limitJunctionStep 原样返回输入电压、不执行任何
// 浮点运算，六个器件文件的数值行为与接线前逐位一致（由 limiting_test.go 的
// golden 摘要与 scripts/regressDiff.py --bit-exact 双重验证）。
// 开启方式（二者等价）：
//   - 程序化：SetIterationLimiting(true, 0.25)，供测试与后续策略对象接线；
//   - 环境变量：CIRCUIT_ITERLIMIT_V=<伏>（>0 开启，0/未设置关闭），供 CLI 级 A/B 复跑。
//
// 环境变量在包 init 时读取一次。本包直接读环境变量是**有意为之的偏差**：
// 引擎配置集中在 load 包解析（element/context.go 的 EngineConfig 注释），
// 而 load/load.go 与 cmd/main.go 在本任务写域之外（K3/K4 归 m6/m5），
// 该偏差与后续迁移点登记在 _results/m4/summary.md。
package semiconductor

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"sync/atomic"
)

const (
	// DEFAULT_LIMIT_VOLTS 默认的单次迭代结电压变化上限（V）：HSPICE/Spectre 常用 0.2~0.3V。
	DEFAULT_LIMIT_VOLTS = 0.25
	// MIN_LIMIT_VOLTS 允许的最小限幅值（V）：小于它等同于逐次迭代硬钳位，会让迭代几乎不前进。
	MIN_LIMIT_VOLTS = 1e-3
	// MAX_LIMIT_VOLTS 允许的最大限幅值（V）：大于它已不能约束 0.7V 级结压跳变，失去限幅意义。
	MAX_LIMIT_VOLTS = 5.0
	// ITERATION_LIMIT_ENV 开启限制的环境变量名（值为限幅伏数；0 或空 = 关闭）。
	ITERATION_LIMIT_ENV = "CIRCUIT_ITERLIMIT_V"
)

// iterationLimiting 当前生效的限制配置（指针为 nil 表示关闭）。
type iterationLimiting struct {
	maxStep float64 // 单次迭代允许的结电压变化上限（V）
}

// iterationLimit 原子读写的生效配置：nil = 关闭（默认值）。
// 用单指针而非"开关 + 数值"两个原子量，避免读侧观察到"已开启但限幅为 0"的中间态。
var iterationLimit atomic.Pointer[iterationLimiting]

// limitingActivations 限制器实际钳位结电压的次数（仅开启时自增；默认关闭时恒为 0）。
var limitingActivations atomic.Uint64

// init 在进程启动时按环境变量应用一次限制配置（未设置 = 保持默认关闭）。
func init() {
	applyIterationLimitEnv(os.Getenv(ITERATION_LIMIT_ENV))
}

// applyIterationLimitEnv 解析环境变量并应用限制；空值/0 = 关闭，非法值 = 保持关闭并告警。
// 非法值不静默吞掉：写到 stderr（不改退出码、不改 stdout 数据），避免"设了却没生效"无人察觉。
func applyIterationLimitEnv(raw string) {
	if raw == "" {
		return
	}
	volts, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[limiting] %s=%q 不是数值，%v；迭代电压限制保持关闭\n", ITERATION_LIMIT_ENV, raw, err)
		return
	}
	if volts == 0 {
		return // 显式关闭
	}
	if err := SetIterationLimiting(true, volts); err != nil {
		fmt.Fprintf(os.Stderr, "[limiting] %s=%q 非法，%v；迭代电压限制保持关闭\n", ITERATION_LIMIT_ENV, raw, err)
	}
}

// SetIterationLimiting 打开或关闭器件迭代电压限制。
// enabled=false 时忽略 maxStep 并关闭；enabled=true 时 maxStep 必须落在
// [MIN_LIMIT_VOLTS, MAX_LIMIT_VOLTS] 内，否则报错且**不改变**当前配置。
func SetIterationLimiting(enabled bool, maxStep float64) error {
	if !enabled {
		iterationLimit.Store(nil)
		return nil
	}
	if math.IsNaN(maxStep) || maxStep < MIN_LIMIT_VOLTS || maxStep > MAX_LIMIT_VOLTS {
		return fmt.Errorf("迭代电压限制步长非法: %v V，允许区间 [%g, %g]", maxStep, float64(MIN_LIMIT_VOLTS), float64(MAX_LIMIT_VOLTS))
	}
	iterationLimit.Store(&iterationLimiting{maxStep: maxStep})
	return nil
}

// IterationLimitingMaxStep 返回当前生效的单次迭代结电压变化上限（V）；0 表示限制关闭。
func IterationLimitingMaxStep() float64 {
	if cfg := iterationLimit.Load(); cfg != nil {
		return cfg.maxStep
	}
	return 0
}

// LimitingActivations 返回进程内限制器实际钳位结电压的次数（默认关闭时恒为 0）。
func LimitingActivations() uint64 {
	return limitingActivations.Load()
}

// ResetLimitingActivations 清零钳位计数，供同一进程内分段统计（测试/对照复跑用）。
func ResetLimitingActivations() {
	limitingActivations.Store(0)
}

// limitJunctionStep 按开关限制本次迭代的结电压：开启时把 newVolt 收敛到
// [oldVolt-maxStep, oldVolt+maxStep] 区间内最近的点，关闭时原样返回 newVolt。
//
// 调用侧约定：oldVolt 必须是上一次迭代**实际存储**的结电压（器件状态槽），
// 且调用发生在器件自身的收敛检查与既有 CircuitJS1 限幅之后——这样最终存储的
// 结电压才真正受界，且关闭时既不引入新的浮点运算也不改变调用顺序。
func limitJunctionStep(newVolt, oldVolt float64) float64 {
	cfg := iterationLimit.Load()
	if cfg == nil {
		return newVolt
	}
	if delta := newVolt - oldVolt; delta > cfg.maxStep {
		limitingActivations.Add(1)
		return oldVolt + cfg.maxStep
	} else if delta < -cfg.maxStep {
		limitingActivations.Add(1)
		return oldVolt - cfg.maxStep
	}
	return newVolt
}

// extraGminActivations 记录「迭代数超过阈值、加性 gmin 被实际启用」的次数。
//
// 它让 D18 类语义修正可被断言：仅当传入的迭代数大于 gminIterThreshold 时自增。
// 计数只增不减，由 ResetExtraGminActivations 清零（与 limitingActivations 同约定）。
var extraGminActivations atomic.Uint64

// gminIterThreshold 是启用加性 gmin 的迭代数阈值。
//
// 语义：**当前时间步内的元件迭代次数**（不是累计时间步数）。调用方必须传
// TimeMNA.CurrentStepElemIterations()——历史上这里误传 GoodIterations()
// （累计成功步数、从不重置），使 ≥500 步的仿真恒被钳到 1e-6（缺陷 D18 / 待办 UD10）。
const gminIterThreshold = 100

// extraGminForIterations 按迭代次数计算加性 gmin。
//
// 依据 CircuitJS1 的收敛辅助思路：迭代越接近上限，就并联越大的到地电导把工作点
// 拉向确定解；上界钳在 1e-6、underflow 下界 1e-15（视为 0）。
// 迭代数不超过 gminIterThreshold 时返回 0（不叠加任何额外电导）。
//
// 返回值同时决定 extraGminActivations 是否自增——这是 D18 的可观测点。
func extraGminForIterations(iterations int) float64 {
	if iterations <= gminIterThreshold {
		return 0
	}
	extraGminActivations.Add(1)
	ratio := 1.0 - math.Min(float64(iterations)/1000.0, 1.0)
	// T4 表驱动登记排除项：这里的指数是迭代限流因子（收敛辅助），不是器件 I-V 的
	// e^(v/NVt)；接入 deviceExp 会把 ~1.9e-6 的插值误差带进限流步长，而该路径默认
	// 关闭、无性能收益。故保留解析 math.Exp（全包仅此一处排除，见 expTable.go 顶部）。
	extraGmin := math.Exp(-12 * math.Ln10 * ratio)
	if extraGmin < 1e-15 {
		return 0
	}
	if extraGmin > 1e-6 {
		return 1e-6
	}
	return extraGmin
}

// ExtraGminActivations 返回「加性 gmin 被启用」的累计次数（D18 可观测点）。
func ExtraGminActivations() uint64 {
	return extraGminActivations.Load()
}

// ResetExtraGminActivations 清零加性 gmin 启用计数（测试与分段统计用）。
func ResetExtraGminActivations() {
	extraGminActivations.Store(0)
}
