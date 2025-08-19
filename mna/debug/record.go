package debug

import (
	"circuit/mna"
	"circuit/types"
	"encoding/json"
	"fmt"
	"io"
)

// Record 记录历史状态
type Record struct {
	Nodes     [][][2]int  // 连接信息
	Elements  []string    // 元件列表
	Current   [][]float64 // 电流列
	Voltage   [][]float64 // 电压列
	Incentive [][]float64 // 激励列
	Time      []float64   // 时间列
}

// Init 初始化
func (list *Record) Init(mna *mna.MNA) {
	eList := make([]string, 0, len(mna.ElementList))
	eList = append(eList, "Gnd")
	m := len(mna.ElementList)
	for i := range m {
		v := mna.ElementList[types.ElementID(i)]
		eList = append(eList, fmt.Sprintf("%s(%d)", v.Type().String(), i+1))
	}
	nList := make([][][2]int, len(mna.NodeList)+1)
	for i := range m {
		v := mna.ElementList[types.ElementID(i)]
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
}

func (Record) IsDebug() bool    { return true }
func (Record) SetDebug(is bool) {}

// Render 格式和输出内容
func (list *Record) Render(w io.Writer) error { return json.NewEncoder(w).Encode(list) }

// Update 记录数据
func (list *Record) Update(mna *mna.MNA) {
	n := len(list.Current)
	// 记录时间
	list.Time = append(list.Time, mna.Time)
	// 记录电压电流
	X := mna.MatX.RawVector().Data
	list.Voltage = append(list.Voltage, append([]float64{}, X[:mna.NumNodes]...))
	list.Incentive = append(list.Incentive, append([]float64{}, mna.MatB.RawVector().Data...))
	list.Current = append(list.Current, append([]float64{}, X[mna.NumNodes:]...))
	// 解析元件电流
	m := len(mna.ElementList)
	for i := range m {
		list.Current[n] = append(list.Current[n], mna.ElementList[i].Current.RawVector().Data...)
	}
}
