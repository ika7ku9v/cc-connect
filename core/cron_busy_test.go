package core

import (
	"strings"
	"testing"
	"time"
)

// T54（2026-11-10）：reuse 模式的 cron／timer 撞上忙碌的 session 會立刻回
// "session is busy"，只寫 log、不重試、不通知。改成等鎖（有上限），最終失敗時通知。

func newBusyTestEngine(t *testing.T) (*Engine, *cujReplyCtxPlatform, *resultAgentSession) {
	t.Helper()
	p := &cujReplyCtxPlatform{stubPlatformEngine: &stubPlatformEngine{n: "test"}}
	as := newResultAgentSession("ok")
	e := NewEngine("test", &resultAgent{session: as}, []Platform{p}, "", LangEnglish)
	return e, p, as
}

func withBusyWait(t *testing.T, poll, cap time.Duration) {
	t.Helper()
	oldPoll, oldCap := sessionBusyPollInterval, sessionBusyWaitCap
	sessionBusyPollInterval, sessionBusyWaitCap = poll, cap
	t.Cleanup(func() { sessionBusyPollInterval, sessionBusyWaitCap = oldPoll, oldCap })
}

func holdSession(e *Engine, key string, releaseAfter time.Duration) {
	s := e.sessions.GetOrCreateActive(key)
	gen, ok := s.TryLock()
	if !ok {
		panic("test setup: session already locked")
	}
	if releaseAfter > 0 {
		go func() {
			time.Sleep(releaseAfter)
			s.Unlock(gen)
		}()
	}
}

func TestCronReuseWaitsForBusySession(t *testing.T) {
	withBusyWait(t, 20*time.Millisecond, 5*time.Second)
	e, _, as := newBusyTestEngine(t)
	key := "test:user1"
	holdSession(e, key, 200*time.Millisecond)

	err := e.ExecuteCronJob(&CronJob{ID: "c1", Project: "test", SessionKey: key, Prompt: "cron prompt"})
	if err != nil {
		t.Fatalf("cron job should wait for the busy session, got error: %v", err)
	}
	if len(as.sentPrompts) != 1 {
		t.Fatalf("agent received %d prompts, want 1", len(as.sentPrompts))
	}
}

func TestTimerReuseWaitsForBusySession(t *testing.T) {
	withBusyWait(t, 20*time.Millisecond, 5*time.Second)
	e, _, as := newBusyTestEngine(t)
	key := "test:user1"
	holdSession(e, key, 200*time.Millisecond)

	err := e.ExecuteTimerJob(&TimerJob{ID: "t1", Project: "test", SessionKey: key, Prompt: "timer prompt"})
	if err != nil {
		t.Fatalf("timer job should wait for the busy session, got error: %v", err)
	}
	if len(as.sentPrompts) != 1 {
		t.Fatalf("agent received %d prompts, want 1", len(as.sentPrompts))
	}
}

func TestTimerReuseGivesUpAfterWaitLimit(t *testing.T) {
	withBusyWait(t, 20*time.Millisecond, 150*time.Millisecond)
	e, _, as := newBusyTestEngine(t)
	key := "test:user1"
	holdSession(e, key, 0) // never released

	start := time.Now()
	err := e.ExecuteTimerJob(&TimerJob{ID: "t2", Project: "test", SessionKey: key, Prompt: "timer prompt"})
	if err == nil || !strings.Contains(err.Error(), "busy") {
		t.Fatalf("want busy error after wait limit, got %v", err)
	}
	if waited := time.Since(start); waited < 150*time.Millisecond || waited > 2*time.Second {
		t.Fatalf("waited %s, want >= wait limit (150ms) and bounded", waited)
	}
	if len(as.sentPrompts) != 0 {
		t.Fatalf("agent received %d prompts, want 0", len(as.sentPrompts))
	}
}

func TestBusyWaitLimitLeavesRoomForTimeout(t *testing.T) {
	withBusyWait(t, time.Second, 15*time.Minute)
	cases := []struct {
		timeout, want time.Duration
	}{
		{0, 15 * time.Minute},
		{30 * time.Minute, 15 * time.Minute},
		{10 * time.Minute, 5 * time.Minute},
		{2 * time.Hour, 15 * time.Minute},
	}
	for _, c := range cases {
		if got := busyWaitLimit(c.timeout); got != c.want {
			t.Errorf("busyWaitLimit(%s) = %s, want %s", c.timeout, got, c.want)
		}
		if c.timeout > 0 && busyWaitLimit(c.timeout) >= c.timeout {
			t.Errorf("busyWaitLimit(%s) must be < timeout", c.timeout)
		}
	}
}

func TestHeartbeatStillFailsFastWhenBusy(t *testing.T) {
	withBusyWait(t, 20*time.Millisecond, 5*time.Second)
	e, _, _ := newBusyTestEngine(t)
	key := "test:user1"
	holdSession(e, key, 0)

	start := time.Now()
	err := e.ExecuteHeartbeat(key, "heartbeat check", true)
	if err == nil || !strings.Contains(err.Error(), "busy") {
		t.Fatalf("heartbeat should fail fast with busy, got %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatalf("heartbeat waited %s, should not wait for the lock", time.Since(start))
	}
}

func runTimerThroughScheduler(t *testing.T, e *Engine, job *TimerJob) {
	t.Helper()
	store, err := NewTimerStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTimerStore: %v", err)
	}
	if err := store.Add(job); err != nil {
		t.Fatalf("store.Add: %v", err)
	}
	sched := NewTimerScheduler(store)
	sched.RegisterEngine("test", e)
	sched.executeJob(job.ID)
}

func TestTimerFailureNotifiesSession(t *testing.T) {
	withBusyWait(t, 20*time.Millisecond, 100*time.Millisecond)
	e, p, _ := newBusyTestEngine(t)
	key := "test:user1"
	holdSession(e, key, 0)

	runTimerThroughScheduler(t, e, &TimerJob{ID: "t3", Project: "test", SessionKey: key,
		Prompt: "acceptance check", Description: "PR 驗收", ScheduledAt: time.Now(), CreatedAt: time.Now()})

	var hit string
	for _, s := range p.getSent() {
		if strings.Contains(s, "PR 驗收") && strings.Contains(s, "t3") {
			hit = s
		}
	}
	if hit == "" {
		t.Fatalf("want a failure notice naming the job, sent=%q", p.getSent())
	}
}

func TestMutedTimerFailureDoesNotNotify(t *testing.T) {
	withBusyWait(t, 20*time.Millisecond, 100*time.Millisecond)
	e, p, _ := newBusyTestEngine(t)
	key := "test:user1"
	holdSession(e, key, 0)

	runTimerThroughScheduler(t, e, &TimerJob{ID: "t4", Project: "test", SessionKey: key,
		Prompt: "quiet", Description: "靜音排程", Mute: true, ScheduledAt: time.Now(), CreatedAt: time.Now()})

	for _, s := range p.getSent() {
		if strings.Contains(s, "t4") {
			t.Fatalf("muted job must not send a failure notice, sent=%q", p.getSent())
		}
	}
}

func TestCronFailureNotifiesSession(t *testing.T) {
	withBusyWait(t, 20*time.Millisecond, 100*time.Millisecond)
	e, p, _ := newBusyTestEngine(t)
	key := "test:user1"
	holdSession(e, key, 0)

	store, err := NewCronStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewCronStore: %v", err)
	}
	job := &CronJob{ID: "c9", Project: "test", SessionKey: key, CronExpr: "0 15 * * *",
		Prompt: "send script", Description: "講稿送稿", Enabled: true, CreatedAt: time.Now()}
	if err := store.Add(job); err != nil {
		t.Fatalf("store.Add: %v", err)
	}
	cs := NewCronScheduler(store)
	cs.RegisterEngine("test", e)
	cs.runJob(job, true)

	found := false
	for _, s := range p.getSent() {
		if strings.Contains(s, "講稿送稿") && strings.Contains(s, "c9") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want a failure notice naming the cron job, sent=%q", p.getSent())
	}
}
