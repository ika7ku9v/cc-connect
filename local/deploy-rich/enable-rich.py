"""在 telegram 平台的 options 段加上 rich_messages = true（已存在就不動）。只改這一行。"""
import sys
import tomllib
from pathlib import Path

p = Path(sys.argv[1])
text = p.read_text()
cfg = tomllib.loads(text)
tg = next(x for x in cfg["projects"][0]["platforms"] if x["type"] == "telegram")
if tg.get("options", {}).get("rich_messages") is True:
    print("rich_messages 已是 true，不變")
    sys.exit(0)
lines = text.splitlines(keepends=True)
idx = next(i for i, l in enumerate(lines) if l.strip() == "[projects.platforms.options]")
indent = lines[idx][: len(lines[idx]) - len(lines[idx].lstrip())] + "  "
lines.insert(idx + 1, f"{indent}rich_messages = true\n")
new = "".join(lines)
check = tomllib.loads(new)  # 寫入前先確認仍是合法 TOML 且值正確
tg2 = next(x for x in check["projects"][0]["platforms"] if x["type"] == "telegram")
assert tg2["options"]["rich_messages"] is True
p.write_text(new)
print("已加入 rich_messages = true")
