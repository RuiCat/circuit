package element

import (
	"runtime"
	"sync"
)

// ParallelOptions 并行盖章选项
type ParallelOptions struct {
	StampWorkers   int     // 盖章工作线程数，<=0 时使用 GOMAXPROCS
	CacheThreshold float64 // 缓存阈值，<=0 则禁用缓存
}

var (
	stampCaches   map[NodeFace]*StampCache // 节点→盖章缓存映射
	cacheTime     float64                  // 当前仿真时间，用于判断是否需要清空缓存
	cacheMu       sync.Mutex               // 缓存访问互斥锁
)

// ResetStampCaches 重置所有盖章缓存
func ResetStampCaches() {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	stampCaches = make(map[NodeFace]*StampCache)
}

// ParallelCallMark 并行执行指定阶段回调
// 根据不同阶段分发处理：Stamp 和 DoStep 采用并行，其余顺序执行
func ParallelCallMark(con *Context, mark Mark, opts ParallelOptions) error {
	cacheMu.Lock()
	if stampCaches == nil {
		stampCaches = make(map[NodeFace]*StampCache)
	}
	curTime := con.CurrentTime()
	if curTime != cacheTime {
		stampCaches = make(map[NodeFace]*StampCache)
		cacheTime = curTime
	}
	cacheMu.Unlock()

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
		return parallelDoStep(con, opts)
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
func parallelDoStep(con *Context, opts ParallelOptions) error {
	workers := opts.StampWorkers
	if workers < 1 {
		workers = runtime.GOMAXPROCS(0)
	}
	useCache := opts.CacheThreshold > 0

	n := len(con.Nodelist)
	if n == 0 {
		return nil
	}

	collectors := make([]*StampCollector, n)
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

				if useCache && elemFace.CanOptimizeStamp() {
					cacheMu.Lock()
					cache := stampCaches[node]
					cacheMu.Unlock()

					if cache != nil && !cache.NeedsBuild() && !cache.HasChanged(con) {
						collector := NewStampCollector(con)
						collector.records = append(collector.records, cache.GetCached()...)
						collectorMu.Lock()
						collectors[idx] = collector
						collectorMu.Unlock()
						continue
					}

					collector := NewStampCollector(con)
					elemFace.DoStep(collector, con.Time, node)

					cacheMu.Lock()
					if cache == nil {
						cache = newStampCache(collector, opts.CacheThreshold)
						stampCaches[node] = cache
					} else {
						cache.Update(collector)
					}
					cacheMu.Unlock()

					collectorMu.Lock()
					collectors[idx] = collector
					collectorMu.Unlock()
				} else {
					collector := NewStampCollector(con)
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
func applyRecord(con *Context, r *RecordedStamp) {
	switch r.Op {
	case OpAdmittance:
		con.StampAdmittance(r.N1, r.N2, r.Value)
	case OpImpedance:
		con.StampImpedance(r.N1, r.N2, r.Value)
	case OpCurrentSource:
		con.StampCurrentSource(r.N1, r.N2, r.Value)
	case OpVoltageSource:
		con.StampVoltageSource(r.N1, r.N2, r.ID1, r.Value)
	case OpVCVS:
		con.StampVCVS(r.N1, r.N2, r.N3, r.N4, r.ID1, r.Value)
	case OpCCCS:
		con.StampCCCS(r.N1, r.N2, r.ID1, r.Value)
	case OpCCVS:
		con.StampCCVS(r.N1, r.N2, r.ID1, r.ID2, r.Value)
	case OpVCCS:
		con.StampVCCS(r.N1, r.N2, r.N3, r.N4, r.Value)
	case OpMatrix:
		con.StampMatrix(r.N1, r.N2, r.Value)
	case OpMatrixSet:
		con.StampMatrixSet(r.N1, r.N2, r.Value)
	case OpRightSide:
		con.StampRightSide(r.N1, r.Value)
	case OpRightSideSet:
		con.StampRightSideSet(r.N1, r.Value)
	case OpUpdateVoltageSource:
		con.UpdateVoltageSource(r.ID1, r.Value)
	case OpIncrementVoltageSource:
		con.IncrementVoltageSource(r.ID1, r.Value)
	}
}

// CanOptimizeStamp 判断该元件类型是否支持盖章优化（缓存）
func (cf *Config) CanOptimizeStamp() bool {
	return cf.CanCacheStamp
}
