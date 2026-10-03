#!/bin/bash
# 部署 cc-connect（Rich Messages 版）。用法：deploy-rich.sh A｜B
#   A：換執行檔，rich_messages 維持關閉
#   B：config 打開 rich_messages（執行檔沿用 A 換好的）
#   C：config 關掉 Telegram 串流預覽（預覽走 editMessageText 舊格式，不關的話一般回覆不會走 Rich）
# 由 systemd-run 在背景執行；之後排 watchdog-rich.sh 檢查，失敗自動還原。
set -euo pipefail
STAGE="${1:?用法：deploy-rich.sh A|B|C}"
D="$HOME/.cc-connect"
BIN=/media/bbclaw/PiData/Project/cc-connect/cc-connect
NEW="$D/cc-connect.rich"
CFG="$D/config.toml"
LOG="$D/deploy-rich.log"
TS=$(date +%Y%m%d-%H%M%S)
exec >>"$LOG" 2>&1
echo "=== $(date -Is) 階段 $STAGE 開始"

same() { [ "$(sha256sum "$1" | cut -d' ' -f1)" = "$(sha256sum "$2" | cut -d' ' -f1)" ]; }

# 備份（每階段各備一份，sha256 必須一致才繼續）
cp -p "$BIN" "$BIN.bak-$STAGE-$TS"; same "$BIN" "$BIN.bak-$STAGE-$TS" || { echo "執行檔備份 sha256 不符，中止"; exit 1; }
cp -p "$CFG" "$CFG.bak-$STAGE-$TS"; same "$CFG" "$CFG.bak-$STAGE-$TS" || { echo "設定檔備份 sha256 不符，中止"; exit 1; }
printf 'BIN_BAK=%s\nCFG_BAK=%s\n' "$BIN.bak-$STAGE-$TS" "$CFG.bak-$STAGE-$TS" > "$D/rich-deploy-$STAGE.state"
echo "備份：$BIN.bak-$STAGE-$TS、$CFG.bak-$STAGE-$TS"

if [ "$STAGE" = A ]; then
  cp -p "$NEW" "$BIN.new-$TS" && mv -f "$BIN.new-$TS" "$BIN"   # 同目錄 rename，原子替換
  same "$NEW" "$BIN" || { echo "新執行檔 sha256 不符，中止"; exit 1; }
  echo "執行檔已換：$(sha256sum "$BIN" | cut -c1-16)"
elif [ "$STAGE" = B ]; then
  python3 "$D/enable-rich.py" "$CFG"
elif [ "$STAGE" = C ]; then
  python3 "$D/disable-preview.py" "$CFG"
elif [ "$STAGE" = D ]; then
  python3 "$D/set-log-level.py" "$CFG" debug   # 暫時開 debug 查文字遺失；查完用 rollback-rich.sh D 改回
elif [ "$STAGE" = E ]; then
  python3 "$D/set-log-level.py" "$CFG" info
elif [ "$STAGE" = CE ]; then
  python3 "$D/disable-preview.py" "$CFG"        # 重新關 Telegram 串流預覽（讓最終回覆走 Rich）
  python3 "$D/set-log-level.py" "$CFG" info     # debug 查完，改回 info
else
  echo "未知階段 $STAGE"; exit 1
fi

SINCE=$(date +%Y-%m-%dT%H:%M:%S)
systemctl --user restart cc-connect
echo "已重啟（SINCE=$SINCE），排 180 秒後的看門狗"
systemd-run --user --on-active=180s --unit="cc-rich-watchdog-$STAGE-$TS" \
  /bin/bash "$D/watchdog-rich.sh" "$STAGE" "$SINCE"
