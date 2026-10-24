// expTable.go —— T4：器件指数求值的表驱动路径（分区查表 + 线性插值），默认关闭。
//
// 热点依据：PN 结的 e^(v/NVt) 在每个牛顿迭代、每个结上至少调用 1 次，是全包唯一的
// 超越函数热点。接入面（deviceExp 是器件侧唯一入口）：
//   - Diode.go 的 safeExp（Transistor.go 的两处调用同走它）；
//   - led.go 的正向线性化与支路电流（原私有 ledSafeExp 已删除）；
//   - zener.go 的正向与齐纳击穿双分支（原私有 zenerSafeExp 已删除）。
//
// 登记排除：limiting.go 的加性 gmin 限流因子 exp(-12·ln10·ratio) 不是器件 I-V，
// 接入只会把插值误差带进限流步长且该路径默认关闭、无性能收益，故保留解析 math.Exp
// （该行有排除注释，供 grep 核对）。
//   - 关闭（默认）：deviceExp == math.Exp(clampExpArg(x))，与改造前各私有 safeExp 逐位一致；
//   - 开启：x∈[-expTableMax, expTableMax] 在等距表上做线性插值，表外（含 ±Inf/NaN）
//     回退解析路径——回退分支与关闭路径逐位相同，故只有表内区间引入插值误差。
//
// 误差上界：线性插值 e^x 的最大相对误差 ≈ h²/8（h = expTableStep = 1/256），
// 即约 1.9e-6。实测最大误差（含每个区间中点这一最坏点）见 expTable_test.go。
//
// 开关语义与 limiting.go 同风格：程序化 SetExpTableEnabled，或进程启动时读环境变量
// CIRCUIT_EXPTABLE（1/true 开启、0/false 关闭、非法值告警并保持关闭）。
// 本包测试的 TestMain 强制关闭，保证既有 golden 用例的前提不被外部环境改变。
package semiconductor

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"sync/atomic"
)

const (
	// EXP_TABLE_ENV 开启表驱动指数的环境变量名（1/true 开启，0/false 关闭）。
	EXP_TABLE_ENV = "CIRCUIT_EXPTABLE"
	// expTableMax 是查表覆盖的指数参数上界：exp(32)≈7.9e13，已覆盖导通区
	// （vd≈0.6~0.8V、1/(N·Vt)≈38.7 ⇒ x≈23~31）；超出即回退解析路径。
	expTableMax = 32.0
	// expTableStep 是表的等距步长（1/256）；线性插值相对误差上界 ≈ step²/8 ≈ 1.9e-6。
	expTableStep = 1.0 / 256.0
	// expTableScale 是步长的倒数（index = (x+max)·scale）。
	expTableScale = 1.0 / expTableStep
	// expTableCount 是表项数：[-max, max] 上含两端的等距点。
	expTableCount = int(2*expTableMax*expTableScale) + 1
	// expClampArg 是解析路径的指数参数钳位（原 safeExp 的 ±700）。
	expClampArg = 700.0
)

// expTableEnabled 是表驱动开关（默认 false = 解析路径）。
var expTableEnabled atomic.Bool

// expTableValues 是预计算的 e^x 表（包初始化时填充）。
var expTableValues [expTableCount]float64

// init 填充查表并应用环境变量开关（未设置 = 保持默认关闭）。
func init() {
	for i := range expTableValues {
		x := -expTableMax + float64(i)*expTableStep
		expTableValues[i] = math.Exp(x)
	}
	applyExpTableEnv(os.Getenv(EXP_TABLE_ENV))
}

// applyExpTableEnv 按环境变量值切换表驱动路径；空值 = 不改动，非法值 = 告警并保持关闭。
func applyExpTableEnv(raw string) {
	if raw == "" {
		return
	}
	enabled, err := strconv.ParseBool(raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[exptable] %s=%q 不是布尔值，%v；表驱动指数保持关闭\n", EXP_TABLE_ENV, raw, err)
		return
	}
	SetExpTableEnabled(enabled)
}

// SetExpTableEnabled 打开或关闭表驱动指数路径（关闭 = 原解析路径，逐位不变）。
func SetExpTableEnabled(enabled bool) {
	expTableEnabled.Store(enabled)
}

// ExpTableEnabled 返回表驱动指数路径当前是否启用。
func ExpTableEnabled() bool {
	return expTableEnabled.Load()
}

// deviceExp 是器件侧指数求值的唯一入口：关闭时解析、开启时查表（表外回退解析）。
func deviceExp(x float64) float64 {
	if !expTableEnabled.Load() {
		return math.Exp(clampExpArg(x))
	}
	return expTableLookup(x)
}

// expTableLookup 在预计算表上做线性插值；表外/NaN 回退解析路径，保证与关闭时一致。
func expTableLookup(x float64) float64 {
	if math.IsNaN(x) || x < -expTableMax || x > expTableMax {
		return math.Exp(clampExpArg(x))
	}
	pos := (x + expTableMax) * expTableScale
	idx := int(pos)
	if idx >= expTableCount-1 {
		// x 恰为上界：pos 可能因舍入落在末项或越过末项。
		return expTableValues[expTableCount-1]
	}
	frac := pos - float64(idx)
	y0 := expTableValues[idx]
	return y0 + (expTableValues[idx+1]-y0)*frac
}

// clampExpArg 把指数参数钳在 [-700, 700]（exp(700)≈1e304，再大即 +Inf）。
func clampExpArg(x float64) float64 {
	if x > expClampArg {
		return expClampArg
	}
	if x < -expClampArg {
		return -expClampArg
	}
	return x
}
