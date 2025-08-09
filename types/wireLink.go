package types

import "slices"

// ElementWire 电路记录信息
type ElementWire struct {
	ElementType  // 元件类型
	*ElementBase // 元件数据
}

// NewElementWire 创建记录
func NewElementWire(id ElementID, t ElementType) *ElementWire {
	ew := &ElementWire{
		ElementType: t,
		ElementBase: &ElementBase{
			Value:    t.InitValue(),
			ID:       id,
			WireList: make(WireList, t.GetPostCount()),
		},
	}
	ew.ElementBase.Init()
	for id := range ew.WireList {
		ew.WireList[id] = ElementHeghWireID
	}
	return ew
}

// WireLink 线路连接
type WireLink struct {
	WireList    map[WireID][]ElementID     // 记录线连接的节点
	ElementList map[ElementID]*ElementWire // 记录节点信息
	NodeCount   ElementID                  // 自增数量
	WireCount   WireID                     // 自增数量
}

// NewWireLink 初始化
func NewWireLink() *WireLink {
	return &WireLink{
		ElementList: make(map[ElementID]*ElementWire),
		WireList:    make(map[WireID][]ElementID),
	}
}

// AddElementID 添加元件
func (wl *WireLink) AddElement(t ElementType) (id ElementID) {
	wl.ElementList[wl.NodeCount] = NewElementWire(wl.NodeCount, t)
	wl.NodeCount++
	return id
}

// DeleteElement 删除元件
func (wl *WireLink) DeleteElement(eid ElementID) {
	if _, ok := wl.ElementList[eid]; !ok {
		return
	}
	ew := wl.ElementList[eid]
	for id, wid := range ew.WireList {
		wl.DeleteWireList(wid, eid, id)
	}
	delete(wl.ElementList, eid)
}

// AddWire 添加线路
func (wl *WireLink) AddWire() WireID {
	wl.WireCount++
	wl.WireList[wl.WireCount] = make([]ElementID, 0)
	return wl.WireCount
}

// AddWireList 添加线路
func (wl *WireLink) AddWireList(wid WireID, eid ElementID, pin int) {
	if _, ok := wl.ElementList[eid]; !ok {
		return
	}
	ew := wl.ElementList[eid]
	if pin < 0 || pin >= len(ew.WireList) {
		return
	}
	if ew.WireList[pin] != ElementHeghWireID {
		wl.DeleteWireList(ew.WireList[pin], eid, pin)
	}
	ew.WireList[pin] = wid
	if slices.Contains(wl.WireList[wid], eid) {
		return
	}
	wl.WireList[wid] = append(wl.WireList[wid], eid)
}

// DeleteWire 删除线路
func (wl *WireLink) DeleteWire(wid WireID) {
	if _, ok := wl.WireList[wid]; !ok {
		return
	}
	for _, eid := range wl.WireList[wid] {
		if ew, ok := wl.ElementList[eid]; ok {
			for i, ewid := range ew.WireList {
				if ewid == wid {
					ew.WireList[i] = ElementHeghWireID
				}
			}
		}
	}
	delete(wl.WireList, wid)
}

// DeleteWireList 删除线路连接
func (wl *WireLink) DeleteWireList(wid WireID, eid ElementID, pin int) {
	if _, ok := wl.ElementList[eid]; !ok {
		return
	}
	ew := wl.ElementList[eid]
	if pin < 0 || pin >= len(ew.WireList) {
		return
	}
	ew.WireList[pin] = ElementHeghWireID
	if slices.Contains(ew.WireList, wid) {
		return
	}
	for c, i := len(wl.WireList[wid]), 0; i < c; i++ {
		if wl.WireList[wid][i] == eid {
			c--
			wl.WireList[wid][i] = wl.WireList[wid][c]
			wl.WireList[wid] = wl.WireList[wid][:c]
			continue
		}
	}
}
