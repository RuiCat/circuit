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
		return con.CallMark(MarkReset)
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
		return con.CallMark(MarkStartIteration)
	case MarkStamp:
		return con.CallMark(MarkStamp)
	case MarkDoStep:
		return con.parallelDoStep()
	case MarkCalculateCurrent:
		return con.CallMark(MarkCalculateCurrent)
	case MarkStepFinished:
		return con.CallMark(MarkStepFinished)
	default:
		return con.CallMark(mark)
	}
}

// parallelDoStep 并行执行 DoStep 阶段
// 将节点列表分片，每个工作线程处理一片，支持可选的盖章缓存优化
func (con *Context) parallelDoStep() error {
	workers := con.ParallelOpts.StampWorkers
	if workers < 1 {
		workers = runtime.GOMAXPROCS(0)
		if workers < 1 {
			workers = 1 // 防御性兜底
		}
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
		end := min(start+chunkSize, n)
		if start >= n {
			break
		}
		wg.Add(1)
		go func(s, e int) {
			defer wg.Done()
			for idx := s; idx < e; idx++ {
				node := con.Nodelist[idx]
				elemFace, ok := getElementFace(node.Base().NodeType)
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
