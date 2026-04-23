package element

import (
	"circuit/mna"
	"math"
)

type StampCache struct {
	nodeSnapshots map[mna.NodeID]float64
	vsrcSnapshots map[mna.VoltageID]float64
	contributions []RecordedStamp
	threshold     float64
	valid         bool
}

func newStampCache(collector *StampCollector, threshold float64) *StampCache {
	return &StampCache{
		nodeSnapshots: make(map[mna.NodeID]float64),
		vsrcSnapshots: make(map[mna.VoltageID]float64),
		contributions: collector.records,
		threshold:     threshold,
		valid:         true,
	}
}

func (sc *StampCache) HasChanged(mna mna.MNAFace[float64]) bool {
	if !sc.valid {
		return true
	}
	for nodeID, snapshot := range sc.nodeSnapshots {
		current := mna.GetNodeVoltage(nodeID)
		if math.Abs(float64(current)-float64(snapshot)) > sc.threshold {
			return true
		}
	}
	for vsrcID, snapshot := range sc.vsrcSnapshots {
		current := mna.GetVoltageSourceCurrent(vsrcID)
		if math.Abs(float64(current)-float64(snapshot)) > sc.threshold {
			return true
		}
	}
	return false
}

func (sc *StampCache) GetCached() []RecordedStamp {
	if !sc.valid {
		return nil
	}
	return sc.contributions
}

func (sc *StampCache) Update(collector *StampCollector) {
	for nodeID := range collector.readNodes {
		sc.nodeSnapshots[nodeID] = collector.inner.GetNodeVoltage(nodeID)
	}
	for vsrcID := range collector.readVsrcs {
		sc.vsrcSnapshots[vsrcID] = collector.inner.GetVoltageSourceCurrent(vsrcID)
	}
	sc.contributions = make([]RecordedStamp, len(collector.records))
	copy(sc.contributions, collector.records)
	sc.valid = true
}

func (sc *StampCache) Invalidate() {
	sc.valid = false
}

func (sc *StampCache) NeedsBuild() bool {
	return !sc.valid || len(sc.contributions) == 0
}
