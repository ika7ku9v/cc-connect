package core

import (
	"fmt"
	"log/slog"
)

// T54（老布 fork，2026-11-10）：排程失敗原本只寫 log，使用者要等到發現「東西沒來」
// 才知道。失敗（等不到鎖、逾時、空回應…）且非 Mute 時，在該 session 送一則通知。

func scheduledJobLabel(description, prompt, exec string) string {
	switch {
	case description != "":
		return description
	case exec != "":
		return truncateStr(exec, 40)
	default:
		return truncateStr(prompt, 40)
	}
}

func (e *Engine) notifyScheduledJobFailure(sessionKey, jobID, label string, cause error) {
	if e == nil || sessionKey == "" {
		return
	}
	if e.i18n == nil {
		// 只有測試用的空殼 Engine 會這樣；正式 NewEngine 一定有 i18n。不送但留紀錄。
		slog.Warn("scheduled job: failure notice skipped (engine has no i18n)", "id", jobID, "session", sessionKey, "error", cause)
		return
	}
	msg := fmt.Sprintf(e.i18n.T(MsgScheduledJobFailed), label, jobID, cause)
	if err := e.SendToSession(sessionKey, msg); err != nil {
		slog.Warn("scheduled job: failure notice not delivered", "id", jobID, "session", sessionKey, "error", err)
	}
}
