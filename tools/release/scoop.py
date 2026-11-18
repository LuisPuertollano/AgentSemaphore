"""Writes the Scoop manifest for one release, from that release's SHA256SUMS.txt.

usage: python3 tools/release/scoop.py <tag, e.g. v0.1.0-alpha.4> <dist folder> > agentsemaphore.json

Install with:  scoop install https://github.com/LuisPuertollano/AgentSemaphore/releases/download/<tag>/agentsemaphore.json
Format: https://github.com/ScoopInstaller/Scoop/wiki/App-Manifests
"""
import json
import pathlib
import re
import sys

REPO = "LuisPuertollano/AgentSemaphore"
ARCHES = {"64bit": "amd64", "arm64": "arm64"}


def manifest(tag: str, sums: dict) -> dict:
    if not re.fullmatch(r"v\d+\.\d+\.\d+(-[0-9A-Za-z.]+)?", tag):
        raise SystemExit(f"not a release tag: {tag}")
    base = f"https://github.com/{REPO}/releases/download/{tag}"
    arch = {}
    for key, goarch in ARCHES.items():
        files = [(f"agentsemaphore-windows-{goarch}.exe", "agentsemaphore.exe"),
                 (f"agentsemaphore-cli-windows-{goarch}.exe", "agentsemaphore-cli.exe")]
        missing = [f for f, _ in files if f not in sums]
        if missing:
            raise SystemExit(f"SHA256SUMS.txt lacks {', '.join(missing)}")
        arch[key] = {"url": [f"{base}/{f}#/{name}" for f, name in files],
                     "hash": [sums[f] for f, _ in files]}
    return {
        "version": tag[1:],
        "description": "A traffic-light dot in the system tray that shows which AI agent needs you",
        "homepage": "https://luispuertollano.github.io/AgentSemaphore/",
        "license": "MIT",
        "architecture": arch,
        "bin": [["agentsemaphore-cli.exe", "agentsemaphore"]],
        "shortcuts": [["agentsemaphore.exe", "AgentSemaphore"]],
        "notes": [
            "Start AgentSemaphore from the Start menu. To start it at sign-in, tick",
            "'Start with the system' in its menu. In a terminal, 'agentsemaphore' runs the console build.",
        ],
    }


def read_sums(dist: pathlib.Path) -> dict:
    sums = {}
    for line in (dist / "SHA256SUMS.txt").read_text().splitlines():
        parts = line.split()
        if len(parts) == 2 and re.fullmatch(r"[0-9a-f]{64}", parts[0]):
            sums[parts[1].lstrip("*")] = parts[0]
    return sums


if __name__ == "__main__":
    if len(sys.argv) != 3:
        raise SystemExit(__doc__)
    json.dump(manifest(sys.argv[1], read_sums(pathlib.Path(sys.argv[2]))), sys.stdout, indent=2)
    sys.stdout.write("\n")
