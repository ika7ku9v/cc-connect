#!/bin/bash
# 撤回階段 C：config 還原成階段 C 之前的備份（重新打開 Telegram 串流預覽），sha256 核對後重啟。
set -uo pipefail
D="$HOME/.cc-connect"
CFG="$D/config.toml"
LOG="$D/deploy-rich.log"
exec >>"$LOG" 2>&1
. "$D/rich-deploy-C.state"
echo "=== $(date -Is) 撤回階段 C（主動，非看門狗）：$CFG_BAK"
cp -p "$CFG_BAK" "$CFG"
if [ "$(sha256sum "$CFG" | cut -d' ' -f1)" != "$(sha256sum "$CFG_BAK" | cut -d' ' -f1)" ]; then
  echo "sha256 不符，不重啟"; exit 1
fi
grep -q '^\[stream_preview\]' "$CFG" && { echo "還原後仍有 [stream_preview]，不重啟"; exit 1; }
SINCE=$(date +%Y-%m-%dT%H:%M:%S)
systemctl --user restart cc-connect
echo "已重啟（SINCE=$SINCE），排 180 秒後的看門狗"
systemd-run --user --on-active=180s --unit="cc-rich-watchdog-revertC-$(date +%H%M%S)" \
  /bin/bash "$D/watchdog-rich.sh" B "$SINCE"
