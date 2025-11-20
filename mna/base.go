package mna

// ElementBase 元件底层数据
type ElementBase struct {
	NetList   *NetList // 原始引用,网表的解析引用
	Pin       []string // 引脚名称
	Value     []any    // 元件数据
	OrigValue []any    // 元件数据备份
	Voltage   []string // 电压源名称
	Internal  []string // 内部引脚名称
}

// Base 得到底层
func (base *ElementBase) Base() *ElementBase { return base }

// PinNum 引脚数量
func (base *ElementBase) PinNum() int { return len(base.Pin) }

// VoltageNum 电压源数量
func (base *ElementBase) VoltageNum() int { return len(base.Voltage) }

// InternalNum 内部数量
func (base *ElementBase) InternalNum() int { return len(base.Internal) }

// ValueNum 元件数据
func (base *ElementBase) ValueNum() int { return len(base.Value) }

// 更新操作 - 将当前值保存到原始值（更新备份）
func (base *ElementBase) Update() {
	copy(base.OrigValue, base.Value)
}

// 回溯操作 - 将原始值恢复到当前值（回滚到备份）
func (base *ElementBase) Rollback() {
	copy(base.Value, base.OrigValue)
}

// Get 得到数据
func (base *ElementBase) Get(i int) any {
	if i >= 0 && i < len(base.Value) {
		return base.Value[i]
	}
	return nil
}

// Set 设置数据
func (base *ElementBase) Set(i int, v any) {
	if i >= 0 && i < len(base.Value) {
		base.Value[i] = v
	}
}
