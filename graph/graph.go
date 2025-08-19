package graph

import (
	"circuit/element/gnd"
	"circuit/element/hegh"
	"circuit/types"
	"fmt"
)

// Graph 连接处理
type Graph struct{ types.ElementGraph }

// NewGraph 创建图
func NewGraph(wireLink *types.WireLink) (graph *Graph, err error) {
	graph = &Graph{
		ElementGraph: types.ElementGraph{
			StampTime: &types.StampTime{
				TimeStep:    types.DefaultTimeStep, // 当前步长
				MaxTimeStep: types.MaxTimeStep,     // 最大允许步长(秒)
				MinTimeStep: types.MinTimeStep,     // 最小允许步长(秒)
			}, // 仿真时间
			StampConfig: &types.StampConfig{
				IsDCAnalysis:  false, // DC分析
				IsTrapezoidal: false, // 梯形法/向后欧拉法
			}, // 仿真参数
			MaxIter:             types.MaxIterations,       // 最大迭代次数
			MaxGoodIter:         types.MaxGoodIter,         // 最大失败数
			NumNodes:            0,                         // 电路节点数量
			NumVoltageSources:   0,                         // 独立电压源数量
			ConvergenceTol:      types.Tolerance,           // 收敛容差
			OscillationCount:    0,                         // 振荡计数器
			OscillationCountMax: types.MaxOscillationCount, // 震荡最大值
		},
	}
	err = graph.Init(wireLink)
	return graph, err
}

// addElement 添加节点
func (graph *Graph) addElement(eid types.ElementID, eleWire *types.ElementWire) error {
	if _, ok := graph.ElementList[eid]; ok {
		return fmt.Errorf("节点重复创建失败: %d", eid)
	}
	e := &types.Element{}
	e.ElementBase = eleWire.ElementBase
	e.ElementFace = eleWire.ElementType.Init(e.ElementBase)
	for i := range eleWire.GetVoltageSourceCnt() {
		e.VoltSource[i] = graph.NumVoltageSources
		graph.NumVoltageSources++
	}
	graph.ElementList[eid] = e
	return nil
}

// Init 初始化
func (graph *Graph) Init(wireLink *types.WireLink) error {
	var isGND bool
	var nodeIDCount int
	nodeList := map[types.NodeID][]types.ElementID{}         // 节点连接
	graph.ElementList = map[types.ElementID]*types.Element{} // 元件列表
	graph.NumVoltageSources = 0
	// 排序
	for wid := range len(wireLink.WireList) {
		wl := wireLink.WireList[wid]
		if wl == nil {
			continue
		}
		// 复位地线标记
		isGND = false
		for _, eid := range wl {
			// 判断节点是否已经创建
			el, ok := graph.ElementList[eid]
			if !ok {
				// 获取节点
				ew := wireLink.ElementList[eid]
				// 跳过对地线的处理
				if ew.ElementType == gnd.Type {
					isGND = true
					continue
				}
				// 创建节点
				if err := graph.addElement(eid, ew); err != nil {
					return err
				}
				// 重置
				el = graph.ElementList[eid]
			}
			// 修正节点索引
			for pinid, elwid := range el.WireList {
				switch elwid {
				case types.ElementHeghWireID: // 如果线路为高阻则设置为高阻节点
					el.Nodes[pinid] = types.ElementHeghNodeID
				case wid: // 如果是当前线路的连接就设置节点为当前节点
					el.Nodes[pinid] = nodeIDCount
				}
			}
			// 添加到元素
			nodeList[nodeIDCount] = append(nodeList[nodeIDCount], eid)
		}
		if !isGND {
			nodeIDCount++
		} else {
			// 递归当前连接
			for _, el := range nodeList[nodeIDCount] {
				ele := graph.ElementList[el]
				// 获取连接并设置当前的节点为地
				for pinid, elwid := range ele.WireList {
					if elwid == wid {
						ele.Nodes[pinid] = types.ElementGndNodeID
					}
				}
			}
			// 当前连接为地需要排除
			nodeList[nodeIDCount] = nodeList[nodeIDCount][:0]
		}
	}
	// 处理其他节点
	nodeCount := wireLink.NodeCount
	for eid := range len(graph.ElementList) {
		ele := graph.ElementList[eid]
		if ele == nil {
			continue
		}
		// 分析节点引脚是否存在悬浮引脚
		for pinID, wid := range ele.WireList {
			if wid == types.ElementHeghWireID {
				// 创建新节点
				e := types.NewElementWire(nodeCount, hegh.Type)
				// 设置交叉索引
				e.Nodes[0] = nodeIDCount
				ele.Nodes[pinID] = nodeIDCount
				// 添加节点
				if err := graph.addElement(nodeCount, e); err != nil {
					return err
				}
				nodeList[nodeIDCount] = []types.ElementID{nodeCount, eid}
				// 更新索引
				nodeCount++
				nodeIDCount++
			}
		}
		// 处理内部节点
		for id := range ele.GetInternalNodeCount() {
			graph.ElementList[eid].ElementBase.SetInternalNode(id, nodeIDCount)
			nodeList[nodeIDCount] = []types.ElementID{eid}
			nodeIDCount++
		}
	}
	// 设置节点数量
	graph.NumNodes = nodeIDCount
	graph.NodeList = make([][]types.ElementID, graph.NumNodes)
	for i := 0; i < nodeIDCount; i++ {
		graph.NodeList[i] = nodeList[i]
	}
	return nil
}
