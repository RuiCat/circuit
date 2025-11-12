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

	// 阶段1: 初始化元件列表并处理地节点
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
			for _, wireID := range ele.WireList {
				mapWireID[wireID] = -1
			}
		}
	}

	// 阶段2: 建立完整的连线到节点映射
	for id := range m {
		ele := wireLink.ElementList[id]
		el := eleList[id]
		if el == nil || el.Type() == types.GndType {
			continue // 跳过无效元件和地
		}

		// 处理所有连线，建立映射
		for _, wireID := range ele.WireList {
			if wireID != types.ElementHeghWireID {
				// 如果连线尚未映射，创建新节点
				if _, exists := mapWireID[wireID]; !exists {
					mapWireID[wireID] = nid
					nid++
				}
			}
		}

		// 处理内部节点
		for range ele.InternalNodes {
			heghNode = append(heghNode, nid)
			nid++
		}
	}

	// 阶段3: 设置元件引脚和连接关系
	for id := range m {
		ele := wireLink.ElementList[id]
		el := eleList[id]
		if el == nil || el.Type() == types.GndType {
			continue // 跳过无效元件和地
		}

		// 构建新元件 ID
		if _, exists := mapEleID[id]; !exists {
			mapEleID[id] = eid
			eid++
		}

		// 设置元件ID
		newID := mapEleID[id]
		el.ID = newID

		// 设置引脚节点
		for i, wireID := range ele.WireList {
			if wireID != types.ElementHeghWireID {
				// 使用预先建立的映射
				el.Nodes[i] = mapWireID[wireID]
			} else {
				// 高阻节点处理
				heghNode = append(heghNode, nid)
				el.Nodes[i] = nid
				nid++
			}
		}

		// 设置内部节点
		for i := range ele.InternalNodes {
			el.ElementBase.SetInternalNode(i, heghNode[0])
			heghNode = heghNode[1:]
		}

		// 处理电压源
		for i := range el.GetVoltageSourceCnt() {
			el.VoltSource[i] = graph.NumVoltageSources
			graph.NumVoltageSources++
		}

		// 记录新元件
		graph.ElementList[newID] = el
	}

	// 阶段4: 建立节点连接关系
	for _, el := range graph.ElementList {
		for _, nodeID := range el.Nodes {
			if nodeID != -1 { // 跳过地节点
				nodeList[nodeID] = append(nodeList[nodeID], el.ID)
			}
		}
	}

	// 处理高阻态节点连接
	for _, nodeID := range heghNode {
		nele := types.NewElementWire(nodeID, types.HeghType)
		el := &types.Element{ID: eid, ElementBase: nele.ElementBase}
		el.ElementFace = nele.ElementType.Init(nele.ElementBase)
		el.Nodes[0] = nodeID
		graph.ElementList[eid] = el
		eid++
		// 添加到节点连接关系
		nodeList[nodeID] = append(nodeList[nodeID], eid-1)
	}

	// 建立节点列表
	graph.NodeList = make([][]types.ElementID, nid)
	for nodeID, elements := range nodeList {
		if nodeID >= 0 && nodeID < nid {
			graph.NodeList[nodeID] = elements
		}
	}

	// 设置节点数
	graph.NumNodes = nid
	return nil
}
