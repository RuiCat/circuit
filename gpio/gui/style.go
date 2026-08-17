// Package gui 提供LCD显示屏的绘图基本功能
package gui

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

// =================================================================================
// 交互状态 — 位掩码形式的UI交互状态，支持多状态组合
// =================================================================================

// State 表示UI元素的交互状态，使用位掩码可同时表达多种状态。
type State uint16

const (
	// StateNone 表示无任何交互状态
	StateNone State = 0
	// StateHovered 表示鼠标或触摸点悬停在元素上方
	StateHovered State = 1 << 0
	// StatePressed 表示元素正在被按下（鼠标按下或触摸按下）
	StatePressed State = 1 << 1
	// StateFocused 表示元素获得键盘或导航焦点
	StateFocused State = 1 << 2
	// StateDisabled 表示元素处于禁用状态，不可交互
	StateDisabled State = 1 << 3
	// StateActive 表示元素处于激活状态（如被点击后的激活态）
	StateActive State = 1 << 4
	// StateSelected 表示元素被选中（如列表项、复选框）
	StateSelected State = 1 << 5
)

// Match 检查当前状态是否包含 mask 中的所有标志位。
// 当 mask 中的所有位都在 s 中被设置时返回 true。
func (s State) Match(mask State) bool {
	return s&mask == mask
}

// Has 检查当前状态是否包含 flag 标志位。
// 只要 flag 中的任意一位在 s 中被设置即返回 true。
func (s State) Has(flag State) bool {
	return s&flag != 0
}

// String 返回人类可读的状态描述字符串。
// 多个状态以 "|" 分隔，无状态时返回 "None"。
func (s State) String() string {
	if s == StateNone {
		return "None"
	}
	var parts []string
	if s&StateHovered != 0 {
		parts = append(parts, "Hovered")
	}
	if s&StatePressed != 0 {
		parts = append(parts, "Pressed")
	}
	if s&StateFocused != 0 {
		parts = append(parts, "Focused")
	}
	if s&StateDisabled != 0 {
		parts = append(parts, "Disabled")
	}
	if s&StateActive != 0 {
		parts = append(parts, "Active")
	}
	if s&StateSelected != 0 {
		parts = append(parts, "Selected")
	}
	return strings.Join(parts, "|")
}

// =================================================================================
// 状态驱动的泛型值 — 根据交互状态动态选择不同的值
// =================================================================================

// Override 表示某个交互状态下对应的值覆盖。
// State 为触发条件，Value 为对应值。
type Override[T any] struct {
	State State
	Value T
}

// Value 是状态驱动的泛型值容器。
// Base 为默认值，Overrides 按优先级排列（后面的优先级更高）。
type Value[T any] struct {
	Base      T
	Overrides []Override[T]
}

// NewValue 创建一个以 base 为默认值的状态驱动值。
func NewValue[T any](base T) Value[T] {
	return Value[T]{Base: base}
}

// On 添加一个状态覆盖并返回一个新的 Value（不可变模式）。
// 在 state 状态下，该 Value 解析为 val。
// 多次调用 On 时，后添加的覆盖优先级更高。
func (v Value[T]) On(state State, val T) Value[T] {
	newOv := make([]Override[T], len(v.Overrides)+1)
	copy(newOv, v.Overrides)
	newOv[len(v.Overrides)] = Override[T]{State: state, Value: val}
	return Value[T]{Base: v.Base, Overrides: newOv}
}

// Resolve 解析当前状态下应使用的目标值。
// 倒序遍历 Overrides，返回第一个匹配的覆盖值；若无匹配则返回 Base。
func (v Value[T]) Resolve(state State) T {
	for i := len(v.Overrides) - 1; i >= 0; i-- {
		if state.Match(v.Overrides[i].State) {
			return v.Overrides[i].Value
		}
	}
	return v.Base
}

// =================================================================================
// 缓动函数 — 为动画提供时间曲线，控制过渡的节奏感
// =================================================================================

// Easing 是缓动函数类型。
// 输入 t 范围为 [0, 1]，表示动画进度；输出为经过缓动变换后的进度值。
type Easing func(t float32) float32

// EaseLinear 线性缓动：匀速过渡。
func EaseLinear(t float32) float32 {
	return t
}

// EaseInQuad 二次缓入：从静止开始加速。
func EaseInQuad(t float32) float32 {
	return t * t
}

// EaseOutQuad 二次缓出：减速到静止。
func EaseOutQuad(t float32) float32 {
	return t * (2 - t)
}

// EaseInOutQuad 二次缓入缓出：先加速后减速。
func EaseInOutQuad(t float32) float32 {
	if t < 0.5 {
		return 2 * t * t
	}
	t = -2*t + 2
	return 1 - t*t/2
}

// EaseInCubic 三次缓入：从静止开始加速（比二次更强）。
func EaseInCubic(t float32) float32 {
	return t * t * t
}

// EaseOutCubic 三次缓出：减速到静止（比二次更强）。
func EaseOutCubic(t float32) float32 {
	t = 1 - t
	return 1 - t*t*t
}

// EaseInOutCubic 三次缓入缓出：先加速后减速（比二次更强）。
func EaseInOutCubic(t float32) float32 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	t = -2*t + 2
	return 1 - t*t*t/2
}

// EaseInQuart 四次缓入：从静止开始加速（非常强烈）。
func EaseInQuart(t float32) float32 {
	return t * t * t * t
}

// EaseOutQuart 四次缓出：减速到静止（非常强烈）。
func EaseOutQuart(t float32) float32 {
	t = 1 - t
	return 1 - t*t*t*t
}

// EaseInOutQuart 四次缓入缓出：先加速后减速（非常强烈）。
func EaseInOutQuart(t float32) float32 {
	if t < 0.5 {
		return 8 * t * t * t * t
	}
	t = -2*t + 2
	return 1 - t*t*t*t/2
}

// EaseOutBounce 弹跳缓出：到达终点前产生多次弹跳效果。
func EaseOutBounce(t float32) float32 {
	const n1 float32 = 7.5625
	const d1 float32 = 2.75

	switch {
	case t < 1/d1:
		return n1 * t * t
	case t < 2/d1:
		t -= 1.5 / d1
		return n1*t*t + 0.75
	case t < 2.5/d1:
		t -= 2.25 / d1
		return n1*t*t + 0.9375
	default:
		t -= 2.625 / d1
		return n1*t*t + 0.984375
	}
}

// EaseOutElastic 弹性缓出：到达终点前产生弹性振动效果。
func EaseOutElastic(t float32) float32 {
	if t == 0 || t == 1 {
		return t
	}
	const c4 = (2 * math.Pi) / 3
	return float32(math.Pow(2, -10*float64(t)))*float32(math.Sin((float64(t*10-0.75))*c4)) + 1
}

// =================================================================================
// 插值器 — 定义如何在两个值之间进行插值
// =================================================================================

// Interpolator 是泛型插值函数类型。
// 给定起始值 a、结束值 b 和进度 t（范围 [0,1]），返回插值结果。
type Interpolator[T any] func(a, b T, t float32) T


// clampF32 将 float32 值限制在 [lo, hi] 范围内。
func clampF32(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// LerpColor 在 RGB565 颜色空间中对两个颜色进行线性插值。
// 分别提取 R（5位）、G（6位）、B（5位）通道，在各通道上插值后重新组合。
// Color 为 uint16 RGB565 格式：位 15-11=R, 位 10-5=G, 位 4-0=B。
func LerpColor(a, b Color, t float32) Color {
	// 提取各通道
	ra := float32((a >> 11) & 0x1F)
	ga := float32((a >> 5) & 0x3F)
	bc := float32(a & 0x1F)

	rb := float32((b >> 11) & 0x1F)
	gb := float32((b >> 5) & 0x3F)
	bd := float32(b & 0x1F)

	// clamp 防止弹性缓动超调导致 uint16 负值溢出
	// 各通道线性插值（夹紧防止缓动函数超调导致溢出）
	r := uint16(clampF32(ra+(rb-ra)*t, 0, 31) + 0.5)
	g := uint16(clampF32(ga+(gb-ga)*t, 0, 63) + 0.5)
	bl := uint16(clampF32(bc+(bd-bc)*t, 0, 31) + 0.5)

	// 重新组合为 RGB565
	return Color((r << 11) | (g << 5) | bl)
}

// LerpFloat32 对两个 float32 值进行线性插值。
func LerpFloat32(a, b float32, t float32) float32 {
	return a + (b-a)*t
}

// LerpInt 对两个 int 值进行线性插值，结果四舍五入。
func LerpInt(a, b int, t float32) int {
	return int(float32(a) + float32(b-a)*t + 0.5)
}

// =================================================================================
// 动画器 — 管理多个属性的动画过渡状态
// =================================================================================

// animEntry 表示单个属性的动画状态。
type animEntry struct {
	from   any
	to     any
	start  time.Time
	dur    time.Duration
	easing Easing
	lerpFn func(any, any, float32) any
	done   bool
}

// Animator 管理一组属性的动画过渡。
// 每个动画由唯一的字符串 id 标识。
type Animator struct {
	// 保护 entries map 的并发访问（渲染循环 vs 输入处理）
	mu      sync.Mutex
	entries map[string]*animEntry
}

// NewAnimator 创建一个新的动画管理器。
func NewAnimator() *Animator {
	return &Animator{
		entries: make(map[string]*animEntry),
	}
}

// AnimateTo 为指定 id 启动一个新的动画过渡。
// from 为起始值，to 为目标值，dur 为动画时长，easing 为缓动函数。
// lerpFn 是类型擦除的插值函数，用于在动画中计算中间值。
func (a *Animator) AnimateTo(id string, from, to any, dur time.Duration, easing Easing, lerpFn func(any, any, float32) any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries[id] = &animEntry{
		from:   from,
		to:     to,
		start:  time.Now(),
		dur:    dur,
		easing: easing,
		lerpFn: lerpFn,
		done:   false,
	}
}

// Resolve 解析指定 id 的动画当前值。
// 如果动画不存在，返回 current（默认值）。
// 如果动画进行中，使用缓动函数和插值器计算当前值。
// 如果动画已完成，返回目标值 to。
func (a *Animator) Resolve(id string, current any) any {
	a.mu.Lock()
	defer a.mu.Unlock()
	entry, ok := a.entries[id]
	if !ok {
		return current
	}

	if entry.done {
		delete(a.entries, id)
		return entry.to
	}

	elapsed := time.Since(entry.start)
	if elapsed >= entry.dur {
		delete(a.entries, id)
		return entry.to
	}

	// 计算缓动进度
	progress := entry.easing(float32(elapsed.Seconds() / entry.dur.Seconds()))
	return entry.lerpFn(entry.from, entry.to, progress)
}

// IsDone 检查指定 id 的动画是否已经结束。
// 如果 id 不存在也返回 true。
func (a *Animator) IsDone(id string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	entry, ok := a.entries[id]
	if !ok {
		return true
	}
	if entry.done {
		delete(a.entries, id)
		return true
	}
	if time.Since(entry.start) >= entry.dur {
		delete(a.entries, id)
		return true
	}
	return false
}

// Cancel 取消指定 id 的动画，移除其状态。
func (a *Animator) Cancel(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.entries, id)
}

// Clear 清除所有动画状态。
func (a *Animator) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = make(map[string]*animEntry)
}

// =================================================================================
// 带动画支持的状态驱动值 — 结合状态覆盖与平滑动画过渡
// =================================================================================

// AnimatedValue 是支持动画过渡的状态驱动值。
// 在 Value[T] 基础上增加了动画时长、缓动函数和插值器配置。
type AnimatedValue[T any] struct {
	Value[T]
	// Duration 表示动画过渡的持续时间
	Duration time.Duration
	// Easing 表示动画使用的缓动函数
	Easing Easing
	// Lerp 表示值之间的插值函数
	Lerp Interpolator[T]
}

// NewAnimatedValue 创建一个带动画支持的状态驱动值。
// base 为默认值，dur 为动画时长，easing 为缓动函数，lerp 为插值函数。
func NewAnimatedValue[T any](base T, dur time.Duration, easing Easing, lerp Interpolator[T]) AnimatedValue[T] {
	return AnimatedValue[T]{
		Value:    NewValue(base),
		Duration: dur,
		Easing:   easing,
		Lerp:     lerp,
	}
}

// On 添加一个状态覆盖并返回新的 AnimatedValue（不可变模式）。
func (av AnimatedValue[T]) On(state State, val T) AnimatedValue[T] {
	av.Value = av.Value.On(state, val)
	return av
}

// Target 获取当前状态下应使用的目标值（不含动画过渡）。
func (av AnimatedValue[T]) Target(state State) T {
	return av.Value.Resolve(state)
}

// Resolve 解析当前状态下经过动画过渡的值。
// anim 为动画管理器，id 为唯一标识符，state 为当前交互状态。
// 动画过程：
//  1. 计算目标值
//  2. 若未设置插值器则直接返回目标值
//  3. 若动画管理器中尚无该 id 的动画，则启动新动画
//  4. 从动画管理器获取当前插值结果
func (av AnimatedValue[T]) Resolve(anim *Animator, id string, state State, currentValue T) T {
	target := av.Target(state)

	// 没有插值器，直接返回目标值
	if av.Lerp == nil {
		return target
	}

	// 如果动画管理器中没有该动画，启动一个新动画
	if anim.IsDone(id) {
		wrap := func(a, b any, t float32) any {
			return av.Lerp(a.(T), b.(T), t)
		}
		anim.AnimateTo(id, currentValue, target, av.Duration, av.Easing, wrap)
	}

	// 从动画管理器获取当前插值结果
	result := anim.Resolve(id, currentValue)
	return result.(T)
}

// =================================================================================
// 默认 UI 样式预设
// =================================================================================

// ButtonStyle 定义按钮的视觉样式。
// 各属性均为状态驱动值，可根据交互状态自动切换外观。
type ButtonStyle struct {
	// BgColor 按钮背景色
	BgColor Value[Color]
	// TextColor 按钮文字颜色
	TextColor Value[Color]
	// BorderColor 按钮边框颜色
	BorderColor Value[Color]
	// CornerRadius 按钮圆角半径
	CornerRadius Value[int]
}

// DefaultButtonStyle 返回默认的按钮样式。
// 包含常见交互状态的视觉反馈：
//
//   - 默认：灰色背景、黑色文字、黑色边框、4px圆角
//   - 悬停：浅灰色背景
//   - 按下：深蓝色背景
//   - 禁用：浅灰色文字
//   - 焦点：蓝色边框
func DefaultButtonStyle() ButtonStyle {
	return ButtonStyle{
		BgColor: NewValue(Gray).
			On(StateHovered, LGray).
			On(StatePressed, DarkBlue),
		TextColor: NewValue(Black).
			On(StateDisabled, LGray),
		BorderColor: NewValue(Black).
			On(StateFocused, Blue),
		CornerRadius: NewValue(4),
	}
}

// String 实现 fmt.Stringer 接口，用于输出 State 的友好描述。
func (o Override[T]) String() string {
	return fmt.Sprintf("Override{State:%s}", o.State)
}
