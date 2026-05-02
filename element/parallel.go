package element

import (
	"circuit/mna"
	"runtime"
	"sync"
)

// ParallelOptions 并行盖章选项
type ParallelOptions struct {
	StampWorkers   int     // 盖章工作线程数，<=0 时使用 GOMAXPROCS
	CacheThreshold float64 // 缓存阈值，<=0 则禁用缓存
}

// ResetStampCaches 重置所有盖章缓存
func (con *Context) ResetStampCaches() {
	con.cacheMu.Lock()
	defer con.cacheMu.Unlock()
	con.stampCaches = make(map[NodeFace]*mna.StampCache)
}

// ParallelCallMark 并行执行指定阶段回调
// 根据不同阶段分发处理：DoStep 采用并行，其余顺序执行
func (con *Context) ParallelCallMark(mark Mark) error {
	con.cacheMu.Lock()
	if con.stampCaches == nil {
		con.stampCaches = make(map[NodeFace]*mna.StampCache)
	}
	curTime := con.CurrentTime()
	if curTime != con.cacheTime {
		con.stampCaches = make(map[NodeFace]*mna.StampCache)
		con.cacheTime = curTime
	}
	con.cacheMu.Unlock()

	switch mark {
	case MarkReset:
		con.CallMark(MarkReset)
		return nil
	case MarkUpdateElements:
		con.UpdateX()
		for i := range con.Nodelist {
			con.Nodelist[i].Base().Update()
		}
		return nil
	case MarkRollbackElements:
		con.RollbackX()
		for i := range con.Nodelist {
			con.Nodelist[i].Base().Rollback()
		}
		return nil
	case MarkStartIteration:
		con.CallMark(MarkStartIteration)
		return nil
	case MarkStamp:
		con.CallMark(MarkStamp)
		return nil
	case MarkDoStep:
		return con.parallelDoStep()
	case MarkCalculateCurrent:
		con.CallMark(MarkCalculateCurrent)
		return nil
	case MarkStepFinished:
		con.CallMark(MarkStepFinished)
		return nil
	default:
		con.CallMark(mark)
		return nil
	}
}

// parallelDoStep 并行执行 DoStep 阶段
// 将节点列表分片，每个工作线程处理一片，支持可选的盖章缓存优化
func (con *Context) parallelDoStep() error {
	workers := con.ParallelOpts.StampWorkers
	if workers < 1 {
		workers = runtime.GOMAXPROCS(0)
	}
	useCache := con.ParallelOpts.CacheThreshold > 0

	n := len(con.Nodelist)
	if n == 0 {
		return nil
	}

	collectors := make([]*mna.StampCollector, n)
	var collectorMu sync.Mutex

	var wg sync.WaitGroup
	chunkSize := (n + workers - 1) / workers
	for w := 0; w < workers; w++ {
		start := w * chunkSize
		end := start + chunkSize
		if end > n {
			end = n
		}
		if start >= n {
			break
		}
		wg.Add(1)
		go func(s, e int) {
			defer wg.Done()
			for idx := s; idx < e; idx++ {
				node := con.Nodelist[idx]

				elemFace, ok := ElementList[node.Base().NodeType]
				if !ok {
					continue
				}

				if useCache && elemFace.IsFlag(FlagCacheStamp) {
					con.cacheMu.Lock()
					cache := con.stampCaches[node]
					con.cacheMu.Unlock()

					if cache != nil && !cache.NeedsBuild() && !cache.HasChanged(con) {
						collector := mna.NewStampCollector(con)
						collector.Records = append(collector.Records, cache.GetCached()...)
						collectorMu.Lock()
						collectors[idx] = collector
						collectorMu.Unlock()
						continue
					}

					collector := mna.NewStampCollector(con)
					elemFace.DoStep(collector, con.Time, node)

					con.cacheMu.Lock()
					if cache == nil {
						cache = mna.NewStampCache(collector, con.ParallelOpts.CacheThreshold)
						con.stampCaches[node] = cache
					} else {
						cache.Update(collector)
					}
					con.cacheMu.Unlock()

					collectorMu.Lock()
					collectors[idx] = collector
					collectorMu.Unlock()
				} else {
					collector := mna.NewStampCollector(con)
					elemFace.DoStep(collector, con.Time, node)
					collectorMu.Lock()
					collectors[idx] = collector
					collectorMu.Unlock()
				}
			}
		}(start, end)
	}
	wg.Wait()

	for _, c := range collectors {
		if c != nil {
			c.Flush(con)
		}
	}

	return nil
}

// applyRecord 根据记录的操作类型，将盖章记录应用到上下文
func applyRecord(con *Context, r *mna.RecordedStamp) {
	switch r.Op {
	case mna.OpAdmittance:
		con.StampAdmittance(r.N1, r.N2, r.Value)
	case mna.OpImpedance:
		con.StampImpedance(r.N1, r.N2, r.Value)
	case mna.OpCurrentSource:
		con.StampCurrentSource(r.N1, r.N2, r.Value)
	case mna.OpVoltageSource:
		con.StampVoltageSource(r.N1, r.N2, r.ID1, r.Value)
	case mna.OpVCVS:
		con.StampVCVS(r.N1, r.N2, r.N3, r.N4, r.ID1, r.Value)
	case mna.OpCCCS:
		con.StampCCCS(r.N1, r.N2, r.ID1, r.Value)
	case mna.OpCCVS:
		con.StampCCVS(r.N1, r.N2, r.ID1, r.ID2, r.Value)
	case mna.OpVCCS:
		con.StampVCCS(r.N1, r.N2, r.N3, r.N4, r.Value)
	case mna.OpMatrix:
		con.StampMatrix(r.N1, r.N2, r.Value)
	case mna.OpMatrixSet:
		con.StampMatrixSet(r.N1, r.N2, r.Value)
	case mna.OpRightSide:
		con.StampRightSide(r.N1, r.Value)
	case mna.OpRightSideSet:
		con.StampRightSideSet(r.N1, r.Value)
	case mna.OpUpdateVoltageSource:
		con.UpdateVoltageSource(r.ID1, r.Value)
	case mna.OpIncrementVoltageSource:
		con.IncrementVoltageSource(r.ID1, r.Value)
	}
}
