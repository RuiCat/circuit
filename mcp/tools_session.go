package mcp

import (
	"fmt"
	"os"

	"circuit/load"
)

// ---------- A 组 · 会话与网表管理 ----------

// sessionResult 会话操作结果。
type sessionResult struct {
	SessionID string       `json:"sessionId"`
	Name      string       `json:"name"`
	Stats     NetlistStats `json:"stats"`
}

// readNetlistArg 解析网表参数：优先 file（文件路径，从磁盘读取），
// 回退 netlist（内联文本）。两者都提供时 file 优先。
// 返回网表文本与错误（文件不存在/读取失败时返回错误）。
func readNetlistArg(args map[string]any) (string, error) {
	if f := argStr(args, "file"); f != "" {
		b, err := os.ReadFile(f)
		if err != nil {
			return "", fmt.Errorf("读取网表文件失败: %w", err)
		}
		return string(b), nil
	}
	n := argStr(args, "netlist")
	if n == "" {
		return "", fmt.Errorf("缺少网表: 请提供 file(文件路径) 或 netlist(网表文本) 参数")
	}
	return n, nil
}

// handleNewSession circuit_new_session：创建仿真会话（可带初始网表）。
// 参数: netlist(可选，网表文本), file(可选，网表文件路径), name(可选)
func (h *Handler) handleNewSession(args map[string]any) (any, error) {
	netlist, err := readNetlistArg(args)
	if err != nil {
		return nil, err
	}
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
// 参数: sessionId(必填), netlist(可选，网表文本), file(可选，网表文件路径)
func (h *Handler) handleLoadNetlist(args map[string]any) (any, error) {
	id := argStr(args, "sessionId")
	if id == "" {
		return nil, errMissing("sessionId")
	}
	netlist, err := readNetlistArg(args)
	if err != nil {
		return nil, err
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
// 参数: netlist(可选，网表文本), file(可选，网表文件路径)
func (h *Handler) handleValidateNetlist(args map[string]any) (any, error) {
	netlist, err := readNetlistArg(args)
	if err != nil {
		return nil, err
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

// Error 返回错误信息文本。
func (e *netlistNotLoadedError) Error() string {
	return "会话 " + e.sessionID + " 尚未加载网表（请先调用 circuit_load_netlist）"
}
