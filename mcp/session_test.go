package mcp

import (
	"strings"
	"testing"
	"time"
)

const rcNetlist = `# RC 充放电测试电路
V1 [1,-1] [0,0,0,0,5]   # DC 5V
R1 [1,2] [1000]         # 1kΩ
C1 [2,-1] [1e-6]        # 1µF
`

const dividerNetlist = `# 分压器
V1 [1,-1] [0,0,0,0,5]
R1 [1,2] [1000]
R2 [2,-1] [2000]
`

const eventNetlist = `# 事件开关电路
V1 [1,-1] [0,0,0,0,5]
B1 [1,2] [SB1, 0.5, 1, 1, 1e6]
R1 [2,-1] [1000]
`

func newTestHandler() (*Handler, *SessionStore) {
	store := NewSessionStore(0)
	return NewHandler(store), store
}

func TestStoreNewGetDelete(t *testing.T) {
	h, store := newTestHandler()
	sess, err := h.store.New("t1", rcNetlist)
	if err != nil {
		t.Fatalf("New 失败: %v", err)
	}
	if sess.ID == "" {
		t.Fatal("会话 ID 为空")
	}
	if sess.Con == nil || len(sess.Con.Nodelist) != 3 {
		t.Fatalf("元件数应为 3，实际 %d", len(sess.Con.Nodelist))
	}
	got, err := store.Get(sess.ID)
	if err != nil || got != sess {
		t.Fatalf("Get 失败: %v", err)
	}
	if err := store.Delete(sess.ID); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	if _, err := store.Get(sess.ID); err == nil {
		t.Fatal("删除后仍能获取会话")
	}
}

func TestStoreNewWithBadNetlist(t *testing.T) {
	_, store := newTestHandler()
	if _, err := store.New("bad", "R1 [1,0] [1000"); err == nil {
		t.Fatal("非法网表应返回错误")
	}
}

func TestLoadNetlistFailureKeepsOld(t *testing.T) {
	h, _ := newTestHandler()
	sess, err := h.store.New("t2", rcNetlist)
	if err != nil {
		t.Fatalf("New 失败: %v", err)
	}
	oldCon := sess.Con
	if err := sess.LoadNetlist("BAD!!!"); err == nil {
		t.Fatal("非法网表应返回错误")
	}
	if sess.Con != oldCon {
		t.Fatal("加载失败后旧上下文应保留")
	}
	if !strings.Contains(sess.Netlist, "V1") {
		t.Fatal("加载失败后旧网表应保留")
	}
}

func TestStoreTTLCleanup(t *testing.T) {
	store := NewSessionStore(30 * time.Millisecond)
	stop := make(chan struct{})
	go store.CleanupLoop(stop)
	sess, err := store.New("ttl", rcNetlist)
	if err != nil {
		t.Fatalf("New 失败: %v", err)
	}
	// 等待 TTL 过期
	time.Sleep(100 * time.Millisecond)
	if _, err := store.Get(sess.ID); err == nil {
		t.Fatal("空闲会话应被 TTL 清理")
	}
	close(stop)
}
