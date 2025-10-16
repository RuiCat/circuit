package motor

import (
	"circuit/types"
)

// Type 元件类型
const (
	DCMotorType types.ElementType = iota + 14
	ACInductionMotorType
	PMSMType
	StepperMotorType
	SeparatelyExcitedMotorType
	ShuntMotorType
	SeriesMotorType
	CompoundMotorType
)

// MotorType 电机类型
type MotorType uint

const (
	DCMotor                MotorType = iota // 直流电机
	ACInductionMotor                        // 交流感应电机
	PMSM                                    // 永磁同步电机
	StepperMotor                            // 步进电机
	SeparatelyExcitedMotor                  // 他励直流电机
	ShuntMotor                              // 并励直流电机
	SeriesMotor                             // 串励直流电机
	CompoundMotor                           // 复励直流电机
)

// Config 默认配置
type Config struct{ Type MotorType }

// Init 初始化
func (c Config) Init(value *types.ElementBase) types.ElementFace {
	switch c.Type {
	case DCMotor:
		return &DCMotorBase{
			ElementBase:  value,
			DCMotorValue: value.Value.(*DCMotorValue),
		}
	case ACInductionMotor:
		return &ACInductionMotorBase{
			ElementBase:           value,
			ACInductionMotorValue: value.Value.(*ACInductionMotorValue),
		}
	case PMSM:
		return &PMSMBase{
			ElementBase: value,
			PMSMValue:   value.Value.(*PMSMValue),
		}
	case StepperMotor:
		return &StepperMotorBase{
			ElementBase:       value,
			StepperMotorValue: value.Value.(*StepperMotorValue),
		}
	case SeparatelyExcitedMotor:
		return &DCMotorBase{
			ElementBase:  value,
			DCMotorValue: value.Value.(*DCMotorValue),
		}
	case ShuntMotor:
		return &ShuntMotorBase{
			ElementBase:     value,
			ShuntMotorValue: value.Value.(*ShuntMotorValue),
		}
	case SeriesMotor:
		return &SeriesMotorBase{
			ElementBase:      value,
			SeriesMotorValue: value.Value.(*SeriesMotorValue),
		}
	case CompoundMotor:
		return &CompoundMotorBase{
			ElementBase:        value,
			CompoundMotorValue: value.Value.(*CompoundMotorValue),
		}
	}
	return nil
}

// InitValue 元件值
func (c Config) InitValue() types.Value {
	switch c.Type {
	case DCMotor, SeparatelyExcitedMotor:
		val := &DCMotorValue{Type: c.Type}
		val.ValueMap = types.ValueMap{}
		return val
	case ACInductionMotor:
		val := &ACInductionMotorValue{Type: c.Type}
		val.ValueMap = types.ValueMap{
			"StatorRes":  0.1,
			"StatorInd":  0.015,
			"RotorRes":   0.15,
			"RotorInd":   0.008,
			"MutualInd":  0.03,
			"Slip":       0.03,
			"Frequency":  50.0,
			"PolePairs":  4,
			"Inertia":    0.1,
			"Damping":    0.01,
			"LoadTorque": 0.1,
		}
		return val
	case PMSM:
		val := &PMSMValue{Type: c.Type}
		val.ValueMap = types.ValueMap{}
		return val
	case StepperMotor:
		val := &StepperMotorValue{Type: c.Type}
		val.ValueMap = types.ValueMap{}
		return val
	case ShuntMotor:
		val := &ShuntMotorValue{Type: c.Type}
		val.ValueMap = types.ValueMap{}
		return val
	case SeriesMotor:
		val := &SeriesMotorValue{Type: c.Type}
		val.ValueMap = types.ValueMap{}
		return val
	case CompoundMotor:
		val := &CompoundMotorValue{Type: c.Type}
		val.ValueMap = types.ValueMap{}
		return val
	}
	return nil
}

// GetPostCount 获取引脚数量
func (c Config) GetPostCount() int {
	switch c.Type {
	case DCMotor:
		return 2
	case ACInductionMotor:
		return 6
	case PMSM:
		return 3
	case StepperMotor:
		return 4
	case SeparatelyExcitedMotor:
		return 4
	case ShuntMotor:
		return 2
	case SeriesMotor:
		return 2
	case CompoundMotor:
		return 3
	}
	return 0
}
