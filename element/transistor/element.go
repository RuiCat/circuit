package transistor

import (
	"circuit/types"
	"fmt"
	"math"
	"strconv"
)

// Type 元件类型
const Type types.ElementType = 8

// 物理常数
const (
	ThermalVoltage = 0.025865 // 电子热电压 (27°C = 300.15K)
)

// 模型标志位
const (
	FLAG_FLIP = 1
)

// Config 默认配置
// 实现 types.ElementConfig 接口
type Config struct{}

// Init 初始化元件实例
// 创建并返回一个新的晶体管元件实例
func (Config) Init(value *types.ElementBase) types.ElementFace {
	return &Base{
		ElementBase: value,
		Value:       value.Value.(*Value),
	}
}

// InitValue 初始化元件值
// 创建并返回一个新的晶体管值实例，带有默认参数
func (Config) InitValue() types.Value {
	val := &Value{
		PNP:       false,
		Beta:      100,
		ModelName: "default",
	}
	val.ValueMap = types.ValueMap{
		"PNP":       false,
		"Beta":      100.0,
		"ModelName": "default",
	}
	return val
}

// GetPostCount 获取引脚数量
// 返回引脚数量(3个引脚：基极、集电极、发射极)
func (Config) GetPostCount() int { return 3 }

// Value 元件值处理结构
// 包含晶体管的可配置参数
type Value struct {
	types.ValueBase         // 基础创建
	PNP             bool    // true为PNP，false为NPN
	Beta            float64 // 电流增益(hFE)
	ModelName       string  // 自定义模型名称
}

// GetVoltageSourceCnt 电压源数量
// 返回0，因为晶体管是被动元件
func (value *Value) GetVoltageSourceCnt() int { return 3 }

// GetInternalNodeCount 内壁引脚数量
// 返回0，因为晶体管没有内部节点
func (value *Value) GetInternalNodeCount() int { return 0 }

// Reset 元件值初始化
// 从值映射中重置晶体管参数
func (value *Value) Reset() {
	val := value.GetValue()
	value.PNP = val["PNP"].(bool)
	value.Beta = val["Beta"].(float64)
	value.ModelName = val["ModelName"].(string)
}

// CirLoad 网表文件写入值
// 从网表格式加载晶体管参数
func (v *Value) CirLoad(value []string) {
	if len(value) >= 1 {
		// 解析PNP标志
		if pnp, err := strconv.ParseBool(value[0]); err == nil {
			v.SetKeyValue("PNP", pnp)
		}
	}
	if len(value) >= 2 {
		// 解析电流增益
		if beta, err := strconv.ParseFloat(value[1], 64); err == nil {
			v.SetKeyValue("Beta", beta)
		}
	}
	if len(value) >= 3 {
		// 解析模型名称
		v.SetKeyValue("ModelName", value[2])
	}
}

// CirExport 网表文件导出值
// 将晶体管参数导出到网表格式
func (value *Value) CirExport() []string {
	return []string{
		fmt.Sprintf("%t", value.PNP),
		fmt.Sprintf("%.6g", value.Beta),
		value.ModelName,
	}
}

// Base 元件实现
// 晶体管元件的主要实现
type Base struct {
	*types.ElementBase
	*Value

	// 晶体管参数
	// node 0 = base
	// node 1 = collector
	// node 2 = emitter
	pnp       int     // 极性因子：1为NPN，-1为PNP
	beta      float64 // 电流增益
	gmin      float64 // 数值稳定性的最小电导
	modelName string  // 模型名称

	// 仿真状态
	lastvbc    float64 // 上次基极-集电极电压
	lastvbe    float64 // 上次基极-发射极电压
	vcrit      float64 // 数值稳定的临界电压
	curcount_c float64 // 集电极电流计数
	curcount_e float64 // 发射极电流计数
	curcount_b float64 // 基极电流计数
	ic         float64 // 集电极电流
	ie         float64 // 发射极电流
	ib         float64 // 基极电流
}

// Type 类型
// 返回元件类型常量
func (base *Base) Type() types.ElementType { return Type }

// Reset 元件值初始化
// 初始化晶体管状态变量
func (base *Base) Reset() {
	base.Value.Reset()
	base.pnp = 1
	if base.PNP {
		base.pnp = -1
	}
	base.beta = base.Beta
	base.modelName = base.ModelName
	base.lastvbc = 0
	base.lastvbe = 0
	base.vcrit = ThermalVoltage * math.Log(ThermalVoltage/(math.Sqrt(2)*1e-13))
	base.curcount_c = 0
	base.curcount_e = 0
	base.curcount_b = 0
	base.ic = 0
	base.ie = 0
	base.ib = 0
}

// StartIteration 迭代开始
// 在每次仿真迭代开始时调用
func (base *Base) StartIteration(stamp types.Stamp) {}

// limitStep 电压步长限制 - 防止数值不稳定
// 实现电压步长限制以防止数值不稳定性
// 这对于SPICE类仿真维持收敛至关重要
func (base *Base) limitStep(vnew, vold float64) float64 {
	// SPICE默认温度27°C(300.15K)下的电子热电压：
	vt := ThermalVoltage
	vcrit := vt * math.Log(vt/(math.Sqrt(2)*1e-13))

	// 应用步长限制以获得数值稳定性
	if vnew > vcrit && math.Abs(vnew-vold) > (vt+vt) {
		if vold > 0 {
			arg := 1 + (vnew-vold)/vt
			if arg > 0 {
				vnew = vold + vt*math.Log(arg)
			} else {
				vnew = vcrit
			}
		} else {
			vnew = vt * math.Log(vnew/vt)
		}
	}
	return vnew
}

// Stamp 更新线性贡献
func (base *Base) Stamp(stamp types.Stamp) {}

// DoStep 执行元件仿真
// 执行此时间步长的主要晶体管仿真计算
func (base *Base) DoStep(stamp types.Stamp) {
	// 从电路节点获取电压
	v1 := stamp.GetVoltage(base.Nodes[0])
	v2 := stamp.GetVoltage(base.Nodes[1])
	v3 := stamp.GetVoltage(base.Nodes[2])
	// 计算电压差 (P/NPN)
	// 基极-集电极电压(通常在活跃模式下为负)
	vbc := float64(base.pnp) * (v1 - v2)
	// 基极-发射极电压(通常在活跃模式下为正)
	vbe := float64(base.pnp) * (v1 - v3)

	// 检查收敛性
	// 检查电压变化是否足够显著以影响收敛
	if math.Abs(vbc-base.lastvbc) > 0.01 || math.Abs(vbe-base.lastvbe) > 0.01 {
		stamp.SetConverged()
	}

	// 为了防止可能的奇异矩阵，在每个P-N结上并联一个小电导
	base.gmin = 1e-12

	// 限制电压步长以保证数值稳定性
	vbc = base.limitStep(vbc, base.lastvbc)
	vbe = base.limitStep(vbe, base.lastvbe)
	base.lastvbc = vbc
	base.lastvbe = vbe

	// SPICE BJT模型参数
	// 默认模型参数
	csat := 1e-13               // 默认饱和电流
	oik := 0.0                  // 默认反向滚降因子
	c2 := 0.0                   // 默认基区漏电流
	vte := 1.0 * ThermalVoltage // 默认发射结发射系数
	oikr := 0.0                 // 默认正向滚降因子
	c4 := 0.0                   // 默认集电结漏电流
	vtc := 1.0 * ThermalVoltage // 默认集电结发射系数

	// 默认参数
	vtn := ThermalVoltage * 1.0 // 发射系数
	evbe, cbe, gbe, cben, gben, evben, evbc, cbc, gbc, cbcn, gbcn, evbcn := 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0
	qb, dqbdve, dqbdvc, q2, sqarg, arg := 0.0, 0.0, 0.0, 0.0, 0.0, 0.0

	// 计算发射结电流
	// 使用肖克利二极管方程计算发射结电流
	if vbe > -5*vtn {
		evbe = math.Exp(vbe / vtn)
		cbe = csat*(evbe-1) + base.gmin*vbe
		gbe = csat*evbe/vtn + base.gmin
		if c2 == 0 {
			cben = 0
			gben = 0
		} else {
			evben = math.Exp(vbe / vte)
			cben = c2 * (evben - 1)
			gben = c2 * evben / vte
		}
	} else {
		gbe = -csat/vbe + base.gmin
		cbe = gbe * vbe
		gben = -c2 / vbe
		cben = gben * vbe
	}

	// 计算集电结电流
	// 使用肖克利二极管方程计算集电结电流
	vtn = ThermalVoltage * 1.0 // 反向发射系数
	if vbc > -5*vtn {
		evbc = math.Exp(vbc / vtn)
		cbc = csat*(evbc-1) + base.gmin*vbc
		gbc = csat*evbc/vtn + base.gmin
		if c4 == 0 {
			cbcn = 0
			gbcn = 0
		} else {
			evbcn = math.Exp(vbc / vtc)
			cbcn = c4 * (evbcn - 1)
			gbcn = c4 * evbcn / vtc
		}
	} else {
		gbc = -csat/vbc + base.gmin
		cbc = gbc * vbc
		gbcn = -c4 / vbc
		cbcn = gbcn * vbc
	}

	// 确定基区电荷项
	// 使用早期电压模型确定基区电荷项
	q1 := 1.0 / (1.0 - 0.0*vbc - 0.0*vbe) // 默认早期电压 = 0
	if oik == 0 && oikr == 0 {
		qb = q1
		dqbdve = q1 * qb * 0.0 // 默认早期电压 = 0
		dqbdvc = q1 * qb * 0.0 // 默认早期电压 = 0
	} else {
		q2 = oik*cbe + oikr*cbc
		arg = math.Max(0, 1+4*q2)
		sqarg = 1.0
		if arg != 0 {
			sqarg = math.Sqrt(arg)
		}
		qb = q1 * (1 + sqarg) / 2
		dqbdve = q1 * (qb*0.0 + oik*gbe/sqarg)  // 默认早期电压 = 0
		dqbdvc = q1 * (qb*0.0 + oikr*gbc/sqarg) // 默认早期电压 = 0
	}

	// 确定直流增量电导
	// 确定直流增量电导
	cc := 0.0
	cex := cbe
	gex := gbe
	cc = cc + (cex-cbc)/qb - cbc/100.0 - cbcn     // 默认beta = 100
	cb := cbe/base.beta + cben + cbc/100.0 + cbcn // 默认beta = 100

	// 计算电流
	// 计算所有端子的最终电流
	base.ic = float64(base.pnp) * cc
	base.ib = float64(base.pnp) * cb
	base.ie = float64(base.pnp) * (-cc - cb)

	// 计算电导
	// 计算矩阵加蓋的增量电导
	gpi := gbe/base.beta + gben
	gmu := gbc/100.0 + gbcn // 默认beta = 100
	go_ := (gbc + (cex-cbc)*dqbdvc/qb) / qb
	gm := (gex-(cex-cbc)*dqbdve/qb)/qb - go_

	// 计算等效电流源
	// 计算矩阵加蓋的等效电流源
	ceqbe := float64(base.pnp) * (cc + cb - vbe*(gm+go_+gpi) + vbc*go_)
	ceqbc := float64(base.pnp) * (-cc + vbe*(gm+go_) - vbc*(gmu+go_))

	// 矩阵加盖
	// Node 0 is the base, node 1 the collector, node 2 the emitter.
	// 为电路求解加蓋雅可比矩阵
	if len(base.Nodes) >= 3 {
		stamp.StampMatrix(base.Nodes[1], base.Nodes[1], gmu+go_)
		stamp.StampMatrix(base.Nodes[1], base.Nodes[0], -gmu+gm)
		stamp.StampMatrix(base.Nodes[1], base.Nodes[2], -gm-go_)
		stamp.StampMatrix(base.Nodes[0], base.Nodes[0], gpi+gmu)
		stamp.StampMatrix(base.Nodes[0], base.Nodes[2], -gpi)
		stamp.StampMatrix(base.Nodes[0], base.Nodes[1], -gmu)
		stamp.StampMatrix(base.Nodes[2], base.Nodes[0], -gpi-gm)
		stamp.StampMatrix(base.Nodes[2], base.Nodes[1], -go_)
		stamp.StampMatrix(base.Nodes[2], base.Nodes[2], gpi+gm+go_)

		// 加盖电流源
		// 在右边向量中加蓋电流源
		stamp.StampRightSide(base.VoltSource[0], -ceqbe-ceqbc) // 第一个电压源
		stamp.StampRightSide(base.VoltSource[1], ceqbc)        // 第二个电压源
		stamp.StampRightSide(base.VoltSource[2], ceqbe)        // 第三个电压源
	}
}

// CalculateCurrent 电流计算
// 计算并存储流入每个引脚的电流
func (base *Base) CalculateCurrent(stamp types.Stamp) {
	// 计算电流
	// 存储所有三个端子(基极、集电极、发射极)的电流
	base.Current.SetVec(0, -base.ib) // 基极电流(负号因为电流流入)
	base.Current.SetVec(1, -base.ic) // 集电极电流(负号因为电流流入)
	base.Current.SetVec(2, -base.ie) // 发射极电流(负号因为电流流入)
}

// StepFinished 步长迭代结束
// 在每次仿真步骤结束时调用
func (base *Base) StepFinished(stamp types.Stamp) {
	// 检查巨大电流
	// 检查可能导致仿真不稳定的过大电流
	if math.Abs(base.ic) > 1e12 || math.Abs(base.ib) > 1e12 {
		fmt.Print("检测到大电流", base.ic, base.ib)
	}
}

// Debug 调试
// 提供晶体管的调试信息
func (base *Base) Debug(stamp types.Stamp) string {
	return ""
}
