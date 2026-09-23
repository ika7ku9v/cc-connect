# RUNBOOK — cc-connect 從零建起（老布的 fork）

> 首次建立：2026-09-23，bbclaw-desktop（Pi 5，aarch64）→ bruce-desktop（Ubuntu 24.04，x86_64）遷移時實跑定案。
> 目標：**新機器照這份從上到下跑一遍，手機 Telegram 就能對到這台的 Friday。**
> 這個 `local/` 目錄是 fork 專用，不送回 upstream。

---

## 0. 架構與這個 fork 多了什麼

```
手機 Telegram ─▶ cc-connect.service (systemd --user, linger)
                   │ ~/.cc-connect/config.toml
                   ▼
                 claude（work_dir = ~/Project/Friday-Agent → 載入 Friday 的 AGENTS.md／hooks）
```

| remote | 指向 | 用途 |
| :-- | :-- | :-- |
| `origin` | `git@github.com:ika7ku9v/cc-connect.git` | 老布的 fork，推自己的分支 |
| `upstream` | `https://github.com/chenhg5/cc-connect.git` | 原作者，只拉 |

分支 `local/usage-timeout` = upstream main + 下列本地改動：

| commit | 內容 | 原因 |
| :-- | :-- | :-- |
| `c6f0fd29` | `core/engine.go` `cmdUsage` 的 timeout 10s → 120s | 抓用量常超過 10s（Pi 時代就有的改動）。`cmdModel` 兩處刻意不動 |
| （本檔） | `local/RUNBOOK.md`、`local/cc-connect.service` | 重建文件與 unit 範本 |

---

## 1. 前置（需要 sudo 的只有這段）

```bash
sudo apt update && sudo apt install -y build-essential   # gcc：go-sqlite3 走 cgo
```

Go：`go.mod` 要求 **>= 1.25**，Ubuntu 24.04 apt 的 `golang-go` 太舊 → 用官方 tarball。
版本與 sha256 查 `https://go.dev/dl/?mode=json`：

```bash
V=go1.27.1   # 換成當下最新版
cd /tmp && curl -fLO https://go.dev/dl/$V.linux-amd64.tar.gz \
  && echo "<sha256>  $V.linux-amd64.tar.gz" | sha256sum -c - \
  && sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf $V.linux-amd64.tar.gz \
  && echo 'export PATH=$PATH:/usr/local/go/bin' | sudo tee /etc/profile.d/go.sh
```

另外要有：Claude Code CLI 已登入（`~/.local/bin/claude`）、tailscale（Taildrop 傳設定用）。

---

## 2. 取得程式碼並編譯

```bash
cd ~/Project
git clone git@github.com:ika7ku9v/cc-connect.git && cd cc-connect
git remote add upstream https://github.com/chenhg5/cc-connect.git
git switch local/usage-timeout

export PATH=$PATH:/usr/local/go/bin
go build -tags 'no_web goolm' \
  -ldflags "-s -w -X main.version=$(sed -n 's/^VERSION := //p' Makefile) -X main.commit=$(git rev-parse --short HEAD) -X main.buildTime=$(date -u '+%Y-%m-%dT%H:%M:%SZ')" \
  -o ~/.local/bin/cc-connect ./cmd/cc-connect
cc-connect --version        # commit 要是 local/usage-timeout 的 HEAD
```

- `no_web`：不編 web 管理介面（config 的 `[management] port = 0`，沒在用）。
  不加這個 tag 會報 `web/embed.go: pattern all:dist: no matching files found`（要先 `make web` 跑 npm）。
- 裝在 `~/.local/bin`：claude 要能呼叫 `cc-connect send` 回傳檔案，這裡在 PATH 內。

---

## 3. 設定檔（不進 git，從舊機器 Taildrop）

舊機器上：

```bash
cd ~ && tar czf cc-connect-conf.tgz .cc-connect/config.toml \
  && tailscale file cp cc-connect-conf.tgz <新機器>: && rm cc-connect-conf.tgz
```

新機器上：

```bash
S=$(mktemp -d) && tailscale file get $S && tar xzf $S/cc-connect-conf.tgz -C $S
install -d -m 700 ~/.cc-connect ~/.cc-connect/tmp
sed "s#work_dir = \".*\"#work_dir = \"$HOME/Project/Friday-Agent\"#" $S/.cc-connect/config.toml > ~/.cc-connect/config.toml
chmod 600 ~/.cc-connect/config.toml
grep -n 'work_dir\|/media\|/home/' ~/.cc-connect/config.toml   # 確認沒有舊機器路徑
rm -rf $S
```

- 平台：Telegram（`token`、`allow_from`、`admin_from` 在 config 裡，**不要貼進對話或 commit**）。
- `agent-prompts/` **不用搬**：cc-connect 每次啟動自己產生。

---

## 4. 信任 usage probe 的暫存目錄（必須老布手動按）

`/usage` 會在 `$TMPDIR` 下開新目錄跑互動式 claude。新目錄會跳 Claude Code 的
「Quick safety check」，**預設選項是「No, exit」**，cc-connect 送 Enter 就等於拒絕並退出。
解法：把 `TMPDIR` 指到一個已信任的目錄（信任會延伸到子目錄，2026-09-23 實測）。

```bash
cd ~/.cc-connect/tmp && claude     # 按 ↓ 選「Yes, I trust this folder」→ Enter → /exit
jq '.projects["'$HOME'/.cc-connect/tmp"].hasTrustDialogAccepted' ~/.claude.json   # 應為 true
```

> 自動化替你按信任對話框會被 Claude Code auto mode 擋下，這一步請人工操作。

---

## 5. systemd user service

```bash
mkdir -p ~/.config/systemd/user
cp ~/Project/cc-connect/local/cc-connect.service ~/.config/systemd/user/
# user 不是 bruce 的話，把檔內的 /home/bruce 換掉
systemd-analyze --user verify ~/.config/systemd/user/cc-connect.service
loginctl enable-linger "$USER"     # 沒登入也開機自啟
```

unit 裡三個 Environment 都不能少：

| 變數 | 少了會怎樣 |
| :-- | :-- |
| `PATH` 含 `~/.local/bin` | 找不到 `claude`／`cc-connect send` |
| `TMPDIR=~/.cc-connect/tmp` | `/usage` 跳信任對話框 → claude 退出 → 沒有回覆 |
| `TERM=xterm-256color` | systemd 預設沒有 TERM → `/usage` 面板解析不到，**卡滿 120s，期間整個 bot 不收訊息** |

---

## 6. 切換（同一個 bot 只能一台連）

同一個 Telegram bot 兩台同時 long polling 會 `409 Conflict` 搶訊息。

```bash
# 舊機器
systemctl --user disable --now cc-connect
# 新機器
systemctl --user enable --now cc-connect
journalctl --user -u cc-connect -f      # 等到 "telegram: connected"、"platform ready"，且沒有 409
```

回滾：新機器 `systemctl --user disable --now cc-connect`，舊機器 `enable --now`。

---

## 7. 驗證（實跑才算完成）

| 手機發 | 預期 | journal 對照 |
| :-- | :-- | :-- |
| `你是誰` | 回「我是 Friday」 | `turn complete`，首次約 15–20s（啟動 claude＋SessionStart hook） |
| `/usage` | 數秒內回覆用量 | `command=usage` 後，下一則訊息立刻被處理（不被阻塞） |

啟動時的 `getMe ... context deadline exceeded`（1s 後重試成功）與 `partial readiness` 是正常現象。

---

## 8. 已知問題

- **upstream 死結（未回報）**：`agent/claudecode/claude_usage.go` 主迴圈若先讀走 `readDone`，
  defer 裡的 `<-readDone` 會永久阻塞 → `/usage` 無回覆、暫存目錄殘留。
  目前靠 §4、§5 讓 probe 不走到這條路；Claude Code 畫面若再改版可能重現。
- **檢查 upstream 有沒有更新**（fork 預設分支是 `local/usage-timeout`）：

  ```bash
  cd ~/Project/cc-connect && git fetch upstream
  git rev-list --count HEAD..upstream/main          # 0 = 沒更新
  git log --oneline HEAD..upstream/main | head -20  # 有更新時看改了什麼
  ```

- **同步 upstream**：`git rebase upstream/main`（在 `local/usage-timeout`）→ 照 §2 重編 →
  `systemctl --user restart cc-connect` → 跑一次 §7 → `git push --force-with-lease origin local/usage-timeout`。
