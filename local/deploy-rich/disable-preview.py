"""在 config.toml 加上頂層 [stream_preview] disabled_platforms = ["telegram"]（已存在就不動）。"""
import sys
import tomllib
from pathlib import Path

p = Path(sys.argv[1])
text = p.read_text()
cfg = tomllib.loads(text)
sp = cfg.get("stream_preview", {})
if "telegram" in sp.get("disabled_platforms", []):
    print("telegram 預覽已停用，不變")
    sys.exit(0)
if "stream_preview" in cfg:
    sys.exit("config 已有 [stream_preview] 段，請手動處理（避免重複表頭）")
new = text.rstrip("\n") + '\n\n[stream_preview]\n  disabled_platforms = ["telegram"]\n'
check = tomllib.loads(new)  # 寫入前確認仍是合法 TOML、其他設定沒變
assert check["stream_preview"]["disabled_platforms"] == ["telegram"]
assert {k: v for k, v in check.items() if k != "stream_preview"} == cfg
p.write_text(new)
print('已加入 [stream_preview] disabled_platforms = ["telegram"]')
