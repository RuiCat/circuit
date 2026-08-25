package mcp

import (
	etime "circuit/element/time"
	"circuit/mna"
	"fmt"
	"strconv"
	"strings"
)

// ---------- C 组 · 电路修改 ----------

// handleSetElementParam circuit_set_element_param：修改元件参数。
// 参数: sessionId(必填), instance(必填), param(必填, 参数名或索引), value(必填, 与参数类型匹配)
// 注意：仿真运行中引擎会读取/回滚元件状态。参数修改仅在以下时机可用：
//   - 无运行中任务；或
//   - 连续模式任务已暂停（circuit_pause）——引擎阻塞在每步开头的同步点，不触碰元件状态，
//     修改后恢复仿真会自动按新参数重新加盖线性元件。
func (h *Handler) handleSetElementParam(args map[string]any) (any, error) {
	sess, err := h.ensureSession(args)
	if err != nil {
		return nil, err
	}
	if j := sess.RunningJob(); j != nil {
		paused := false
		if j.Continuous {
			if tm, ok := sess.Con.Time.(*etime.TimeMNA); ok {
				paused = tm.Status() == mna.StatusPaused
			}
		}
		if !paused {
			return nil, fmt.Errorf("会话 %s 有运行中的任务 %s，请先暂停（circuit_pause）或取消后再修改参数", sess.ID, j.ID)
		}
	}
	inst := argStr(args, "instance")
	if inst == "" {
		return nil, errMissing("instance")
	}
	param := argStr(args, "param")
	if param == "" {
		return nil, errMissing("param")
	}
	newVal, hasVal := args["value"]
	if !hasVal {
		return nil, errMissing("value")
	}

	var elem *elementNodeRef
	for _, e := range sess.Con.Nodelist {
		if instanceName(e) == inst {
			elem = &elementNodeRef{e}
			break
		}
	}
	if elem == nil {
		return nil, fmt.Errorf("元件 %s 不在网表中（可用 circuit_list_elements 查看）", inst)
	}

	// 定位参数索引：名称优先，其次数字索引
	cfg := elem.Config()
	idx := -1
	if n, err := strconv.Atoi(param); err == nil {
		if n >= 0 && n < len(cfg.ValueName) {
			idx = n
		} else {
			return nil, fmt.Errorf("参数索引 %d 越界（元件 %s 共 %d 个参数）", n, inst, len(cfg.ValueName))
		}
	} else {
		for i, name := range cfg.ValueName {
			if name == param {
				idx = i
				break
			}
		}
		if idx < 0 {
			return nil, fmt.Errorf("元件 %s 无参数 %q（可用参数: %s）", inst, param, strings.Join(cfg.ValueName, ", "))
		}
	}

	old := elem.Get(idx)
	newTyped, err := coerceParam(newVal, old)
	if err != nil {
		return nil, fmt.Errorf("参数 %q 需要类型 %s: %v", cfg.ValueName[idx], paramType(old), err)
	}
	if err := elem.Set(idx, newTyped); err != nil {
		return nil, err
	}
	// 连续模式暂停中修改参数：请求恢复仿真时重新加盖线性元件（矩阵用新参数）。
	if j := sess.RunningJob(); j != nil && j.Continuous {
		if tm, ok := sess.Con.Time.(*etime.TimeMNA); ok {
			tm.InvalidateLinearStamp()
		}
	}
	return map[string]any{
		"instance": inst,
		"param":    cfg.ValueName[idx],
		"index":    idx,
		"oldValue": old,
		"newValue": newTyped,
	}, nil
}

// handleSetEvent circuit_set_event：设置事件值（运行中任务也可调用）。
// 参数: sessionId(必填), event(必填, 事件名), value(必填, number/boolean/string)
func (h *Handler) handleSetEvent(args map[string]any) (any, error) {
	sess, err := h.ensureSession(args)
	if err != nil {
		return nil, err
	}
	event := argStr(args, "event")
	if event == "" {
		return nil, errMissing("event")
	}
	v, hasVal := args["value"]
	if !hasVal {
		return nil, errMissing("value")
	}
	switch v.(type) {
	case float64, int, bool, string:
	default:
		return nil, fmt.Errorf("事件值仅支持 number/boolean/string，收到 %T", v)
	}
	if err := sess.SetEvent(event, v); err != nil {
		return nil, err
	}
	return map[string]any{"event": event, "value": v}, nil
}

// handleSetSimParams circuit_set_sim_params：设置仿真默认参数。
// 参数: sessionId(必填), initialStep?/minStep?/maxStep?/absTol?/relTol?/maxIter?/maxSteps?/parallel?
func (h *Handler) handleSetSimParams(args map[string]any) (any, error) {
	sess, err := h.store.Get(argStr(args, "sessionId"))
	if err != nil {
		return nil, err
	}
	sess.mu.Lock()
	defer sess.mu.Unlock()
	cfg := sess.SimCfg
	set := map[string]bool{}
	if v, ok := argFloat(args, "initialStep"); ok {
		if v <= 0 {
			return nil, fmt.Errorf("initialStep 必须 > 0")
		}
		cfg.InitialStep = v
		set["initialStep"] = true
	}
	if v, ok := argFloat(args, "minStep"); ok {
		if v <= 0 {
			return nil, fmt.Errorf("minStep 必须 > 0")
		}
		cfg.MinStep = v
		set["minStep"] = true
	}
	if v, ok := argFloat(args, "maxStep"); ok {
		if v <= 0 {
			return nil, fmt.Errorf("maxStep 必须 > 0")
		}
		cfg.MaxStep = v
		set["maxStep"] = true
	}
	if v, ok := argFloat(args, "absTol"); ok {
		if v <= 0 {
			return nil, fmt.Errorf("absTol 必须 > 0")
		}
		cfg.AbsTol = v
		set["absTol"] = true
	}
	if v, ok := argFloat(args, "relTol"); ok {
		if v <= 0 {
			return nil, fmt.Errorf("relTol 必须 > 0")
		}
		cfg.RelTol = v
		set["relTol"] = true
	}
	if v, ok := argInt(args, "maxIter"); ok {
		if v <= 0 {
			return nil, fmt.Errorf("maxIter 必须 > 0")
		}
		cfg.MaxIter = v
		set["maxIter"] = true
	}
	if v, ok := argInt(args, "maxSteps"); ok {
		if v <= 0 {
			return nil, fmt.Errorf("maxSteps 必须 > 0")
		}
		cfg.MaxSteps = v
		set["maxSteps"] = true
	}
	if v, ok := argInt(args, "parallel"); ok {
		if v < 0 {
			return nil, fmt.Errorf("parallel 不能为负")
		}
		cfg.Parallel = v
		set["parallel"] = true
	}
	if cfg.MinStep > cfg.MaxStep {
		return nil, fmt.Errorf("minStep(%g) 不能大于 maxStep(%g)", cfg.MinStep, cfg.MaxStep)
	}
	sess.SimCfg = cfg
	return map[string]any{
		"simCfg":  cfg,
		"updated": sortedKeys(set),
	}, nil
}

// handleSetTrigger circuit_set_trigger：设置时间触发点列表。
// 参数: sessionId(必填), triggers[](必填, 时间点数组，单位秒)
// 触发点在下次任务启动时应用；运行中不可修改。
func (h *Handler) handleSetTrigger(args map[string]any) (any, error) {
	sess, err := h.ensureSession(args)
	if err != nil {
		return nil, err
	}
	if j := sess.RunningJob(); j != nil {
		return nil, fmt.Errorf("会话 %s 有运行中的任务 %s，请等待完成或取消后再设置触发点", sess.ID, j.ID)
	}
	times := argFloatSlice(args, "triggers")
	if times == nil {
		return nil, errMissing("triggers")
	}
	triggers := make([]mna.Trigger, 0, len(times))
	for _, t := range times {
		if t <= 0 {
			return nil, fmt.Errorf("触发时间必须 > 0，收到 %g", t)
		}
		triggers = append(triggers, mna.Trigger{Time: t})
	}
	sess.mu.Lock()
	sess.triggers = triggers
	// 若会话已有时间控制器（上次仿真遗留）同步应用；任务启动时会重新应用
	if sess.Con != nil && sess.Con.Time != nil {
		sess.Con.Time.SetTriggers(triggers)
	}
	sess.mu.Unlock()
	return map[string]any{"triggers": triggers, "count": len(triggers)}, nil
}

// handleSetEngineConfig circuit_set_engine_config：设置引擎仿真配置（EngineCfg）。
// 参数: sessionId(必填), 各配置项可选:
//   sparseLU?(bool)    强制稀疏 LU（CIRCUIT_LUSPARSE）
//   forceDense?(bool)  强制稠密 LU（CIRCUIT_LUSDENSE，覆盖自动选择；与 sparseLU 互斥）
//   gminCont?(bool)    Gmin 延续步进（CIRCUIT_GMCONT，解决大数字电路 t=0 奇异）
//   naturalGmin?(float64) 自然 gmin（CIRCUIT_NATGMIN）
//   gminFinal?(float64)   延续终值（CIRCUIT_GMINFINAL，<=0 用默认）
//   globalGmin?(float64)  全局对角 gmin（CIRCUIT_GLOBALGMIN，0=关闭）
//   gminDbg?(bool)     打印 gmin 步进轨迹（CIRCUIT_GMIN_DBG）
//   noEq?(bool)        跳过行/列均衡化（CIRCUIT_NOEQ）
// 生效时机：下次仿真任务启动时读取（TransientSimulation 每次调用时从 con.EngineCfg 读取）。
// 与 circuit_set_sim_params（步长/容差等数值仿真参数）互补。
func (h *Handler) handleSetEngineConfig(args map[string]any) (any, error) {
	sess, err := h.ensureSession(args)
	if err != nil {
		return nil, err
	}
	if j := sess.RunningJob(); j != nil {
		return nil, fmt.Errorf("会话 %s 有运行中的任务 %s，请等待完成或取消后再设置引擎配置", sess.ID, j.ID)
	}
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if sess.Con == nil {
		return nil, fmt.Errorf("会话 %s 尚未加载网表", sess.ID)
	}
	cfg := sess.Con.EngineCfg
	old := cfg
	set := map[string]bool{}
	if v, ok := argBool(args, "sparseLU"); ok {
		cfg.SparseLU = v
		set["sparseLU"] = true
	}
	if v, ok := argBool(args, "forceDense"); ok {
		cfg.ForceDense = v
		set["forceDense"] = true
	}
	if v, ok := argBool(args, "gminCont"); ok {
		cfg.GminCont = v
		set["gminCont"] = true
	}
	if v, ok := argFloat(args, "naturalGmin"); ok {
		if v < 0 {
			return nil, fmt.Errorf("naturalGmin 不能为负")
		}
		cfg.NaturalGmin = v
		set["naturalGmin"] = true
	}
	if v, ok := argFloat(args, "gminFinal"); ok {
		if v < 0 {
			return nil, fmt.Errorf("gminFinal 不能为负")
		}
		cfg.GminFinal = v
		set["gminFinal"] = true
	}
	if v, ok := argFloat(args, "globalGmin"); ok {
		if v < 0 {
			return nil, fmt.Errorf("globalGmin 不能为负")
		}
		cfg.GlobalGmin = v
		set["globalGmin"] = true
	}
	if v, ok := argBool(args, "gminDbg"); ok {
		cfg.GminDbg = v
		set["gminDbg"] = true
	}
	if v, ok := argBool(args, "noEq"); ok {
		cfg.NoEq = v
		set["noEq"] = true
	}
	sess.Con.EngineCfg = cfg
	changed := map[string]any{}
	if cfg != old {
		for _, k := range sortedKeys(set) {
			switch k {
			case "sparseLU":
				changed[k] = cfg.SparseLU
			case "forceDense":
				changed[k] = cfg.ForceDense
			case "gminCont":
				changed[k] = cfg.GminCont
			case "naturalGmin":
				changed[k] = cfg.NaturalGmin
			case "gminFinal":
				changed[k] = cfg.GminFinal
			case "globalGmin":
				changed[k] = cfg.GlobalGmin
			case "gminDbg":
				changed[k] = cfg.GminDbg
			case "noEq":
				changed[k] = cfg.NoEq
			}
		}
	}
	return map[string]any{
		"engineConfig": map[string]any{
			"sparseLU":    cfg.SparseLU,
			"forceDense":  cfg.ForceDense,
			"gminCont":    cfg.GminCont,
			"naturalGmin": cfg.NaturalGmin,
			"gminFinal":   cfg.GminFinal,
			"globalGmin":  cfg.GlobalGmin,
			"gminDbg":     cfg.GminDbg,
			"noEq":        cfg.NoEq,
		},
		"updated": sortedKeys(set),
		"changed": changed,
	}, nil
}
