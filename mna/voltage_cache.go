package mna

import (
	"math"
)

// StampCache MNA加盖缓存，用于缓存非线性元件的加盖结果
// 通过监测节点电压和电压源电流的变化，判断缓存是否仍然有效，避免重复加盖计算
type StampCache struct {
	NodeSnapshots map[NodeID]float64     // 节点电压快照，用于检测电压变化
	VsrcSnapshots map[VoltageID]float64  // 电压源电流快照，用于检测电流变化
	Contributions []RecordedStamp        // 缓存的加盖记录
	Threshold     float64                // 变化检测阈值，超过此值视为发生变化
	Valid         bool                   // 缓存是否有效
}

// NewStampCache 创建新的加盖缓存
// collector: 加盖收集器，用于获取加盖记录
// threshold: 变化检测阈值
func NewStampCache(collector *StampCollector, threshold float64) *StampCache {
	contributions := make([]RecordedStamp, len(collector.Records))
	copy(contributions, collector.Records)
	return &StampCache{
		NodeSnapshots: make(map[NodeID]float64),
		VsrcSnapshots: make(map[VoltageID]float64),
		Contributions: contributions,
		Threshold:     threshold,
		Valid:         true,
	}
}

// HasChanged 检查节点电压或电压源电流是否发生了超过阈值的变化
// 如果缓存无效或任意监测量的变化超过阈值，返回 true
func (sc *StampCache) HasChanged(mna MNAFace[float64]) bool {
	if !sc.Valid {
		return true
	}
	for nodeID, snapshot := range sc.NodeSnapshots {
		current := mna.GetNodeVoltage(nodeID)
		if math.Abs(float64(current)-float64(snapshot)) > sc.Threshold {
			return true
		}
	}
	for vsrcID, snapshot := range sc.VsrcSnapshots {
		current := mna.GetVoltageSourceCurrent(vsrcID)
		if math.Abs(float64(current)-float64(snapshot)) > sc.Threshold {
			return true
		}
	}
	return false
}

// GetCached 获取缓存的加盖记录
// 如果缓存无效，返回 nil；否则返回缓存的加盖记录
func (sc *StampCache) GetCached() []RecordedStamp {
	if !sc.Valid {
		return nil
	}
	return sc.Contributions
}

// Update 更新缓存快照和加盖记录
// 从collector中读取当前节点电压和电压源电流快照，并复制最新的加盖记录
func (sc *StampCache) Update(collector *StampCollector) {
	for nodeID := range collector.ReadNodes {
		sc.NodeSnapshots[nodeID] = collector.Inner.GetNodeVoltage(nodeID)
	}
	for vsrcID := range collector.ReadVsrcs {
		sc.VsrcSnapshots[vsrcID] = collector.Inner.GetVoltageSourceCurrent(vsrcID)
	}
	sc.Contributions = make([]RecordedStamp, len(collector.Records))
	copy(sc.Contributions, collector.Records)
	sc.Valid = true
}

// Invalidate 使缓存失效
// 下一次访问时将重新计算加盖
func (sc *StampCache) Invalidate() {
	sc.Valid = false
}

// NeedsBuild 判断是否需要重新构建加盖
// 当缓存无效或加盖记录为空时，需要重新构建
func (sc *StampCache) NeedsBuild() bool {
	return !sc.Valid || len(sc.Contributions) == 0
}
