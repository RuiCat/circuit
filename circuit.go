package circuit

import (
	"bufio"
	"circuit/graph"
	"circuit/mna"
	"circuit/types"
	"math"

	"fmt"

	"os"
	"strconv"
	"strings"
	"unicode"

	_ "circuit/element"
)

// Circuit 电路模拟器
type Circuit struct {
	*types.WireLink
}

// NewCircuit 初始化
func NewCircuit() *Circuit {
	return &Circuit{WireLink: types.NewWireLink()}
}

// Load 加载 netlist 格式数据
func (cir *Circuit) Load(filename string) (err error) {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		// 解析标记
		if fields[0][0] == '.' {
			continue
		}
		var t types.ElementType = types.TypeUnknown
		var v int
		for i, s := range fields[0] {
			if unicode.IsNumber(s) {
				t = types.GetNameType(fields[0][:i])
				v, err = strconv.Atoi(fields[0][i:])
				if err != nil {
					return err
				}
				continue
			}
		}
		if t == types.TypeUnknown {
			return fmt.Errorf("未知元件定义解析: %s", line)
		}
		// 处理引脚
		l := t.GetPostCount() + 1
		if len(fields) < l {
			return fmt.Errorf("元件定义引脚定义错误: %d-%s", t.GetPostCount(), line)
		}
		// 创建元件
		eid, i := types.ElementID(v), 1
		cir.ElementList[eid] = types.NewElementWire(eid, t)
		for ; i < l; i++ {
			wid, err := strconv.Atoi(fields[i])
			if err != nil {
				return err
			}
			cir.AddWireList(types.WireID(wid), eid, i-1)
		}
		// 原始元件值
		value, l := []string{}, len(fields)
		for ; i < l; i++ {
			if fields[i] != "" && fields[i][0] != '#' {
				value = append(value, fields[i])
			}
		}
		cir.ElementList[eid].Value.CirLoad(value)
	}
	cir.NodeCount = types.ElementID(len(cir.ElementList))
	return scanner.Err()
}

// Export 导出 netlist 格式数据
func (cir *Circuit) Export(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	// 导出组件
	for eid, ewl := range cir.ElementList {
		writer.WriteString(ewl.ElementType.String())
		fmt.Fprint(writer, eid)
		writer.WriteRune(' ')
		for _, wid := range ewl.WireList {
			fmt.Fprint(writer, wid)
			writer.WriteRune(' ')
		}
		for _, v := range ewl.ElementBase.CirExport() {
			writer.WriteString(v)
			writer.WriteRune(' ')
		}
		writer.WriteRune('\n')
	}
	writer.Flush()
	return nil
}

// MNA 得到节点电压计算结构体
func (c *Circuit) MNA() (*mna.MNA, error) {
	g, err := graph.NewGraph(c.WireLink)
	if err != nil {
		return nil, err
	}
	return mna.NewMNA(g), nil
}

// Simulate 进行仿真
func Simulate(endTime float64, mna *mna.MNA) error {
	// 初始化调试
	mna.Debug.Init(mna)
	// 主时间循环
	var goodIterations int
	var maxGoodIter int
	mna.Time = 0
	mna.GoodIterations = 0 // 成功次数
	for mna.Time <= endTime {
		// 范围动态调整
		switch {
		case mna.TimeStep > mna.MaxTimeStep: // 超出上限
			mna.TimeStep = math.Max(mna.TimeStep/1.5, mna.MinTimeStep)
		case mna.TimeStep < mna.MinTimeStep: // 超出下限
			mna.TimeStep = math.Min(mna.TimeStep*1.2, mna.MaxTimeStep)
		case goodIterations > 10 && mna.TimeStep < mna.MaxTimeStep: // 尝试增加步进长度
			oldTimeStep := mna.TimeStep
			mna.TimeStep = math.Min(mna.TimeStep*1.2, mna.MaxTimeStep)
			if mna.TimeStep != oldTimeStep {
				mna.StampUP()
				goodIterations = 0 // 重置计数器
				maxGoodIter = 0
			}
		case goodIterations < -5 && mna.TimeStep > mna.MinTimeStep: // 尝试减少步进长度
			oldTimeStep := mna.TimeStep
			mna.TimeStep = math.Max(mna.TimeStep/1.5, mna.MinTimeStep)
			if mna.TimeStep != oldTimeStep {
				mna.StampUP()
				goodIterations = 0 // 重置计数器
				maxGoodIter = 0
			}
		case maxGoodIter > types.MaxIterations: // 达到最大错误数量
			return fmt.Errorf("到达最大错误数量")
		case mna.Time > endTime: // 结束位限制
			return nil
		}
		// 计算矩阵
		if ok, err := mna.Solve(); err != nil {
			return err
		} else if ok {
			// 更新步进
			goodIterations++
			// 递归次数
			mna.GoodIterations++
			// 推进时间
			mna.Time += mna.TimeStep
		} else {
			// 失败不更新
			goodIterations--
			maxGoodIter++
		}
	}
	return nil
}
