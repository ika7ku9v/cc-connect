package core

import (
	"strings"
	"testing"
	"time"
)

// session_mode=new_active：timer 觸發時等舊 active session 空閒 → 照 /new 清掉 →
// 建新 active session → 把 prompt 當第一則訊息送進去。給 agent 自己換 session 接力用。

func TestTimerNewActiveModeValidation(t *testing.T) {
	for _, m := range []string{"new_active", "new-active"} {
		j := &TimerJob{ID: "v1", Project: "p", SessionKey: "k", Prompt: "x", SessionMode: m, ScheduledAt: time.Now()}
		if err := validateTimerJob(j); err != nil {
			t.Errorf("timer should accept session_mode %q, got %v", m, err)
		}
		if !j.UsesNewActiveSession() {
			t.Errorf("UsesNewActiveSession() = false for %q", m)
		}
		if j.UsesNewSessionPerRun() {
			t.Errorf("UsesNewSessionPerRun() must be false for %q", m)
		}
	}
	c := &CronJob{ID: "c", Project: "p", SessionKey: "k", CronExpr: "0 9 * * *", Prompt: "x", SessionMode: "new-active"}
	if err := validateCronJob(c); err == nil {
		t.Errorf("cron must reject session_mode new-active")
	}
}

func seedOldSession(e *Engine, key string) *Session {
	old := e.sessions.GetOrCreateActive(key)
	old.AddHistory("user", "old question")
	old.AddHistory("assistant", "old answer")
	old.SetAgentSessionID("old-agent-session", "claudecode")
	return old
}

func TestTimerNewActiveReplacesActiveSession(t *testing.T) {
	withBusyWait(t, 20*time.Millisecond, 5*time.Second)
	e, _, as := newBusyTestEngine(t)
	key := "test:user1"
	old := seedOldSession(e, key)
	oldID := e.sessions.ActiveSessionID(key)

	err := e.ExecuteTimerJob(&TimerJob{ID: "h1", Project: "test", SessionKey: key,
		Prompt: "read handoff and continue", SessionMode: "new-active"})
	if err != nil {
		t.Fatalf("new_active timer failed: %v", err)
	}
	newID := e.sessions.ActiveSessionID(key)
	if newID == "" || newID == oldID {
		t.Fatalf("active session not replaced: old=%q new=%q", oldID, newID)
	}
	if old.HistoryLen() != 0 {
		t.Errorf("old session history not cleared: len=%d", old.HistoryLen())
	}
	if old.GetAgentSessionID() != "" {
		t.Errorf("old agent session id not cleared: %q", old.GetAgentSessionID())
	}
	if len(as.sentPrompts) != 1 || !strings.Contains(as.sentPrompts[0], "read handoff") {
		t.Fatalf("agent prompts = %q, want exactly the handoff prompt", as.sentPrompts)
	}
	if got := e.sessions.GetOrCreateActive(key).HistoryLen(); got < 1 {
		t.Errorf("new active session has no history, prompt did not run in it (len=%d)", got)
	}
}

func TestTimerNewActiveWaitsForBusyOldSession(t *testing.T) {
	withBusyWait(t, 20*time.Millisecond, 5*time.Second)
	e, _, as := newBusyTestEngine(t)
	key := "test:user1"
	seedOldSession(e, key)
	oldID := e.sessions.ActiveSessionID(key)
	holdSession(e, key, 200*time.Millisecond)

	err := e.ExecuteTimerJob(&TimerJob{ID: "h2", Project: "test", SessionKey: key,
		Prompt: "continue", SessionMode: "new_active"})
	if err != nil {
		t.Fatalf("should wait for the busy old session, got %v", err)
	}
	if e.sessions.ActiveSessionID(key) == oldID {
		t.Fatalf("active session not replaced after waiting")
	}
	if len(as.sentPrompts) != 1 {
		t.Fatalf("agent received %d prompts, want 1", len(as.sentPrompts))
	}
}

func TestTimerNewActiveLeavesOldSessionWhenBusyForever(t *testing.T) {
	withBusyWait(t, 20*time.Millisecond, 150*time.Millisecond)
	e, _, as := newBusyTestEngine(t)
	key := "test:user1"
	old := seedOldSession(e, key)
	oldID := e.sessions.ActiveSessionID(key)
	holdSession(e, key, 0)

	err := e.ExecuteTimerJob(&TimerJob{ID: "h3", Project: "test", SessionKey: key,
		Prompt: "continue", SessionMode: "new_active"})
	if err == nil || !strings.Contains(err.Error(), "busy") {
		t.Fatalf("want busy error, got %v", err)
	}
	if e.sessions.ActiveSessionID(key) != oldID {
		t.Errorf("active session must not change when the old one never frees up")
	}
	if old.HistoryLen() != 2 || old.GetAgentSessionID() != "old-agent-session" {
		t.Errorf("old session must be untouched: history=%d agent=%q", old.HistoryLen(), old.GetAgentSessionID())
	}
	if len(as.sentPrompts) != 0 {
		t.Fatalf("agent received %d prompts, want 0", len(as.sentPrompts))
	}
}
