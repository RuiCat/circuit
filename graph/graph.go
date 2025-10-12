package graph

import (
	"circuit/types"
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

		},
	}
	err = graph.Init(wireLink)
	return graph, err
}

// Init 初始化
func (graph *Graph) Init(wireLink *types.WireLink) error {
	// 定义映射
	mapEleID := map[types.ElementID]types.ElementID{}
	mapWireID := map[types.WireID]types.NodeID{}
	var (
		eid      types.ElementID = 0
		nid      types.NodeID    = 0
		heghNode []types.NodeID  = []types.NodeID{}
	)
	// 处理
	nodeList := map[types.NodeID][]types.ElementID{}
	graph.ElementList = map[types.ElementID]*types.Element{}
	graph.NumVoltageSources = 0
	// 初始化元件列表
	eleList := map[types.ElementID]*types.Element{}
	m := types.ElementID(len(wireLink.ElementList))
	for id := range m {
		ele := wireLink.ElementList[id]
		// 跳过没有连接的节点
		if len(ele.WireList) == 0 {
			continue
		}
		el := &types.Element{ID: -1, ElementBase: ele.ElementBase}
		el.ElementFace = ele.ElementType.Init(ele.ElementBase)
		eleList[id] = el
		// 处理地
		if el.Type() == types.GndType {
			for _, id := range ele.WireList {
				mapWireID[id] = -1
			}
		}
	}
	// 处理连接
	for id := range m {
		ele := wireLink.ElementList[id]
		// 获取节点
		el := eleList[id]
		if el.Type() == types.GndType {
			continue // 跳过地
		}
		// 构建新节点 ID
		if _, ok := mapEleID[id]; !ok {
			mapEleID[id] = eid
			eid++
		}
		// 设置ID
		id = mapEleID[id]
		el.ID = id
		// 处理连接
		for i, id := range ele.WireList {
			if id != types.ElementHeghWireID {
				// 处理连接的节点
				if _, ok := mapWireID[id]; !ok {
					mapWireID[id] = nid
					nid++
				}
				el.Nodes[i] = mapWireID[id]
			} else {
				// 未连接节点设置为高阻
				heghNode = append(heghNode, nid)
				el.Nodes[i] = nid
				nid++
			}
			// 设置映射
			nodeList[el.Nodes[i]] = append(nodeList[el.Nodes[i]], el.ID)
		}
		// 设置内部节点
		for i := range ele.InternalNodes {
			heghNode = append(heghNode, nid)
			nodeList[nid] = append(nodeList[nid], el.ID)
			el.ElementBase.SetInternalNode(i, nid)
			nid++
		}
		// 处理电压源
		for i := range el.GetVoltageSourceCnt() {
			el.VoltSource[i] = graph.NumVoltageSources
			graph.NumVoltageSources++
		}
		// 记录新元素
		graph.ElementList[id] = el
	}
	// 处理高组态节点连接
	for _, v := range heghNode {
		nele := types.NewElementWire(v, types.HeghType)
		el := &types.Element{ID: eid, ElementBase: nele.ElementBase}
		el.ElementFace = nele.ElementType.Init(nele.ElementBase)
		el.Nodes[0] = v
		graph.ElementList[eid] = el
		eid++
	}
	// 互相连接
	graph.NodeList = make([][]types.ElementID, len(nodeList)-1)
	for i, l := range nodeList {
		if i != -1 {
			graph.NodeList[i] = l
		}
	}
	// 设置节点数
	graph.NumNodes = len(graph.NodeList)
	return nil
}
