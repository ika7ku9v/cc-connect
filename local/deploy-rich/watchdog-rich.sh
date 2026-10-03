#!/bin/bash
# 部署後檢查；不通過就還原該階段的備份。用法：watchdog-rich.sh <階段> <SINCE>
set -uo pipefail
STAGE="$1"; SINCE="$2"
D="$HOME/.cc-connect"
LOG="$D/deploy-rich.log"
exec >>"$LOG" 2>&1
echo "=== $(date -Is) 看門狗 階段 $STAGE（SINCE=$SINCE）"

active=$(systemctl --user is-active cc-connect)
read -r connected errors < <(python3 - "$D/logs/cc-connect.log" "$SINCE" <<'PY'
import re, sys
path, since = sys.argv[1], sys.argv[2]
conn = err = 0
for line in open(path, errors="replace"):
    m = re.match(r"time=(\S{19})", line)
    if not m or m.group(1) < since:
        continue
    if 'msg="telegram: connected"' in line:
        conn += 1
    if "level=ERROR" in line and "telegram" in line.lower():
        err += 1
print(conn, err)
PY
)
echo "is-active=$active connected=$connected telegram_errors=$errors"

if [ "$active" = active ] && [ "${connected:-0}" -ge 1 ] && [ "${errors:-1}" -eq 0 ]; then
  echo "通過"
  exit 0
fi
echo "未通過 → 還原階段 $STAGE"
/bin/bash "$D/rollback-rich.sh" "$STAGE"
