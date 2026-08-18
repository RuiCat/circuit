package mcp

import (
	"circuit/load"
)

// ---------- A 组 · 会话与网表管理 ----------

// sessionResult 会话操作结果。
type sessionResult struct {
	SessionID string       `json:"sessionId"`
	Name      string       `json:"name"`
	Stats     NetlistStats `json:"stats"`
}

// handleNewSession circuit_new_session：创建仿真会话（可带初始网表）。
// 参数: netlist(可选), name(可选)
func (h *Handler) handleNewSession(args map[string]any) (any, error) {
	netlist := argStr(args, "netlist")
	name := argStr(args, "name")
	sess, err := h.store.New(name, netlist)
	if err != nil {
		return nil, err
	}
	res := sessionResult{SessionID: sess.ID, Name: sess.Name}
	if sess.Con != nil {
		res.Stats = collectStats(sess.Con)
	}
	return res, nil
}

// handleLoadNetlist circuit_load_netlist：加载/替换会话网表。
// 参数: sessionId(必填), netlist(必填)
func (h *Handler) handleLoadNetlist(args map[string]any) (any, error) {
	id := argStr(args, "sessionId")
	if id == "" {
		return nil, errMissing("sessionId")
	}
	netlist := argStr(args, "netlist")
	if netlist == "" {
		return nil, errMissing("netlist")
	}
	sess, err := h.store.Get(id)
	if err != nil {
		return nil, err
	}
	if err := sess.LoadNetlist(netlist); err != nil {
		return nil, netlistError(err)
	}
	res := sessionResult{SessionID: sess.ID, Name: sess.Name}
	res.Stats = collectStats(sess.Con)
	return res, nil
}

// handleValidateNetlist circuit_validate_netlist：校验网表，不创建会话。
// 参数: netlist(必填)
func (h *Handler) handleValidateNetlist(args map[string]any) (any, error) {
	netlist := argStr(args, "netlist")
	if netlist == "" {
		return nil, errMissing("netlist")
	}
	con, err := load.LoadString(netlist)
	if err != nil {
		return map[string]any{
			"valid": false,
			"error": netlistError(err).Error(),
		}, nil
	}
	con.InitResumeCond()
	return map[string]any{
		"valid": true,
		"stats": collectStats(con),
	}, nil
}

// handleListSessions circuit_list_sessions：列出会话。
func (h *Handler) handleListSessions(args map[string]any) (any, error) {
	return map[string]any{"sessions": h.store.List()}, nil
}

// handleDeleteSession circuit_delete_session：删除会话。
// 参数: sessionId(必填)
func (h *Handler) handleDeleteSession(args map[string]any) (any, error) {
	id := argStr(args, "sessionId")
	if id == "" {
		return nil, errMissing("sessionId")
	}
	if err := h.store.Delete(id); err != nil {
		return nil, err
	}
	return map[string]any{"deleted": true, "sessionId": id}, nil
}

// Handler 工具处理器集合，持有会话存储。
type Handler struct {
	store *SessionStore
}

// NewHandler 创建工具处理器。
func NewHandler(store *SessionStore) *Handler {
	return &Handler{store: store}
}

// ensureSession 获取会话（供各工具复用）。
func (h *Handler) ensureSession(args map[string]any) (*Session, error) {
	id := argStr(args, "sessionId")
	if id == "" {
		return nil, errMissing("sessionId")
	}
	sess, err := h.store.Get(id)
	if err != nil {
		return nil, err
	}
	if sess.Con == nil {
		return nil, errNoNetlist(sess.ID)
	}
	return sess, nil
}

func errNoNetlist(id string) error {
	return &netlistNotLoadedError{sessionID: id}
}

// netlistNotLoadedError 会话未加载网表错误。
type netlistNotLoadedError struct {
	sessionID string
}

func (e *netlistNotLoadedError) Error() string {
	return "会话 " + e.sessionID + " 尚未加载网表（请先调用 circuit_load_netlist）"
}
