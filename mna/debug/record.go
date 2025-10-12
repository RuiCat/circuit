package debug

import (
	"circuit/types"
	"encoding/json"
	"fmt"
	"io"
	"log"
)

// Record 记录历史状态
type Record struct {
	Nodes      [][][2]int  // 连接信息
	Elements   []string    // 元件列表
	Current    [][]float64 // 电流列
	CurrentStr []string    // 电流信息
	Voltage    [][]float64 // 电压列
	Incentive  [][]float64 // 激励列
	Time       []float64   // 时间列
}

// Init 初始化
func (list *Record) Init(mna types.Stamp) {
	graph := mna.GetGraph()
	eList := make([]string, 0, len(graph.ElementList))
	eList = append(eList, "Gnd")
	m := types.ElementID(len(graph.ElementList))
	for i := 0; i < m; i++ {
		v := graph.ElementList[i]
		eList = append(eList, fmt.Sprintf("%s(%d)", v.Type().String(), i+1))
	}
	nList := make([][][2]int, len(graph.NodeList)+1)
	for i := 0; i < m; i++ {
		v := graph.ElementList[i]
		for l, n := range v.Nodes {
			n += 1
			nList[n] = append(nList[n], [2]int{int(i + 1), int(l)})
		}
		for l, n := range v.InternalNodes {
			n += 1
			nList[n] = append(nList[n], [2]int{int(i + 1), int(l)})
		}
	}
	list.Elements = eList
	list.Nodes = nList
	for id := range graph.NumVoltageSources {
		list.CurrentStr = append(list.CurrentStr, fmt.Sprintf("电压源(%d)", id))
	}
	for i := 0; i < m; i++ {
		ele := graph.ElementList[i]
		for j := 0; j < ele.Current.Len(); j++ {
			list.CurrentStr = append(list.CurrentStr, fmt.Sprintf("%s-%d(%d)", ele.Type(), ele.ID, j))
		}
	}
}

func (Record) IsDebug() bool    { return true }
func (Record) SetDebug(is bool) {}

// Render 格式和输出内容
func (list *Record) Render(w io.Writer) error { return json.NewEncoder(w).Encode(list) }

// Update 记录数据
func (list *Record) Update(mna types.Stamp) {
	graph := mna.GetGraph()
	n := len(list.Current)
	// 记录时间
	list.Time = append(list.Time, graph.Time)
	// 记录电压电流
	X := mna.GetX()
	list.Voltage = append(list.Voltage, append([]float64{}, X[:graph.NumNodes]...))
	list.Incentive = append(list.Incentive, append([]float64{}, mna.GetB()...))
	list.Current = append(list.Current, append([]float64{}, X[graph.NumNodes:]...))
	// 解析元件电流
	m := len(graph.ElementList)
	for i := range m {
		list.Current[n] = append(list.Current[n], graph.ElementList[i].Current.RawVector().Data...)
	}
}

func (list *Record) Error(err error) { log.Println(err) }
