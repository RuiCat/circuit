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
func (c *Circuit) MNA() (m types.MNA, _ error) {
	g, err := graph.NewGraph(c.WireLink)
	if err != nil {
		return nil, err
	}
	m = mna.NewMNA(g)
	if m == nil {
		return nil, fmt.Errorf("矩阵始化失败")
	}
	return m, nil
}

// Simulate 进行仿真
func Simulate(endTime float64, mna types.MNA) error {
	// 初始化调试
	graph := mna.GetGraph()
	if graph.Debug != nil {
		graph.Debug.Init(mna)
	}
	// 主时间循环
	var goodIterations int
	var maxGoodIter int
	graph.Time = 0
	graph.GoodIterations = 0 // 成功次数
	for graph.Time <= endTime {
		// 范围动态调整
		switch {
		case graph.TimeStep > graph.MaxTimeStep: // 超出上限
			graph.TimeStep = math.Max(graph.TimeStep/1.5, graph.MinTimeStep)
		case graph.TimeStep < graph.MinTimeStep: // 超出下限
			graph.TimeStep = math.Min(graph.TimeStep*1.2, graph.MaxTimeStep)
		case goodIterations > 10 && graph.TimeStep < graph.MaxTimeStep: // 尝试增加步进长度
			oldTimeStep := graph.TimeStep
			graph.TimeStep = math.Min(graph.TimeStep*1.2, graph.MaxTimeStep)
			if graph.TimeStep != oldTimeStep {
				mna.StampUP()
				goodIterations = 0 // 重置计数器
				maxGoodIter = 0
			}
		case goodIterations < -5 && graph.TimeStep > graph.MinTimeStep: // 尝试减少步进长度
			oldTimeStep := graph.TimeStep
			graph.TimeStep = math.Max(graph.TimeStep/1.5, graph.MinTimeStep)
			if graph.TimeStep != oldTimeStep {
				mna.StampUP()
				goodIterations = 0 // 重置计数器
				maxGoodIter = 0
			}
		case maxGoodIter > types.MaxIterations: // 达到最大错误数量
			return fmt.Errorf("到达最大错误数量")
		case graph.Time > endTime: // 结束位限制
			return nil
		}
		// 计算矩阵
		if ok, err := mna.Solve(); err != nil {
			return err
		} else if ok {
			// 更新步进
			goodIterations++
			// 递归次数
			graph.GoodIterations++
			// 推进时间
			graph.Time += graph.TimeStep
		} else {
			// 失败不更新
			goodIterations--
			maxGoodIter++
		}
	}
	return nil
}
