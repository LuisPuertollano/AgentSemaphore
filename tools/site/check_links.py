"""Checks that every local src/href in the assembled landing page exists.

usage: python3 tools/site/check_links.py <site folder>
"""
import pathlib
import re
import sys

root = pathlib.Path(sys.argv[1])
bad = []
for page in root.rglob("*.html"):
    for ref in re.findall(r'(?:src|href)="([^"]+)"', page.read_text(encoding="utf-8")):
        if re.match(r"^(https?:|mailto:|#)", ref):
            continue
        target = (page.parent / ref.split("#")[0]).resolve()
        if not target.exists():
            bad.append(f"{page.relative_to(root)}: {ref}")
for b in bad:
    print("missing:", b)
print(f"{'FAILED' if bad else 'ok'}: local links checked in {root}")
sys.exit(1 if bad else 0)
