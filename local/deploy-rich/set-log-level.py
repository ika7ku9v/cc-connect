"""把 config.toml 的 [log] level 改成指定值（只改這一行）。用法：set-log-level.py <config> <level>"""
import re
import sys
import tomllib
from pathlib import Path

p, level = Path(sys.argv[1]), sys.argv[2]
text = p.read_text()
cfg = tomllib.loads(text)
if cfg.get("log", {}).get("level") == level:
    print(f"log level 已是 {level}，不變")
    sys.exit(0)
lines = text.splitlines(keepends=True)
start = next(i for i, l in enumerate(lines) if l.strip() == "[log]")
for i in range(start + 1, len(lines)):
    if lines[i].lstrip().startswith("["):
        sys.exit("[log] 段裡找不到 level，請手動處理")
    if re.match(r"\s*level\s*=", lines[i]):
        indent = lines[i][: len(lines[i]) - len(lines[i].lstrip())]
        lines[i] = f'{indent}level = "{level}"\n'
        break
new = "".join(lines)
check = tomllib.loads(new)
assert check["log"]["level"] == level
assert {k: v for k, v in check.items() if k != "log"} == {k: v for k, v in cfg.items() if k != "log"}
p.write_text(new)
print(f"log level 已改成 {level}")
