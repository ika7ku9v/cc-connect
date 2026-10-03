#!/bin/bash
# 還原某階段部署前的執行檔與設定檔（sha256 核對），重啟並通知。用法：rollback-rich.sh A|B
set -uo pipefail
STAGE="${1:?用法：rollback-rich.sh A|B|C}"
D="$HOME/.cc-connect"
BIN=/media/bbclaw/PiData/Project/cc-connect/cc-connect
CFG="$D/config.toml"
LOG="$D/deploy-rich.log"
exec >>"$LOG" 2>&1
# shellcheck disable=SC1090
. "$D/rich-deploy-$STAGE.state"
echo "=== $(date -Is) 還原階段 $STAGE：$BIN_BAK、$CFG_BAK"
cp -p "$BIN_BAK" "$BIN.restore" && mv -f "$BIN.restore" "$BIN"
cp -p "$CFG_BAK" "$CFG"
ok=1
[ "$(sha256sum "$BIN" | cut -d' ' -f1)" = "$(sha256sum "$BIN_BAK" | cut -d' ' -f1)" ] || ok=0
[ "$(sha256sum "$CFG" | cut -d' ' -f1)" = "$(sha256sum "$CFG_BAK" | cut -d' ' -f1)" ] || ok=0
systemctl --user restart cc-connect
sleep 20
state=$(systemctl --user is-active cc-connect)
echo "還原 sha256 一致=$ok，重啟後 is-active=$state"
/media/bbclaw/PiData/Project/PodcastMaker/tools/notify.sh -m "⚠️ cc-connect Rich Messages 部署（階段 $STAGE）沒通過檢查，已自動還原舊版（sha256 一致=$ok，服務狀態 $state）。紀錄：~/.cc-connect/deploy-rich.log" || echo "通知失敗"
