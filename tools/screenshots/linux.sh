#!/usr/bin/env bash
# Real screenshots of the tray on a Linux desktop panel (XFCE), for the README
# and the website. Runs on a virtual display with fake Claude Code sessions
# whose names are neutral examples; nothing of the real user is read.
#
# Needs: Xvfb, xfce4-panel, xfconf, dbus-run-session, xdotool, ImageMagick.
# usage: tools/screenshots/linux.sh <agentsemaphore> <fakeagent> <out dir>
set -euo pipefail
if [ "${1:-}" = --inner ]; then INNER=1; shift; else INNER=0; fi
EXE=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
FAKE=$(cd "$(dirname "$2")" && pwd)/$(basename "$2")
mkdir -p "$3"; OUT=$(cd "$3" && pwd)
HERE=$(cd "$(dirname "$0")" && pwd)

if [ "$INNER" = 0 ]; then
  W=$(mktemp -d)
  export HOME="$W/home" TMPDIR="$W/tmp" LANG=en_US.UTF-8 LANGUAGE=en
  unset XDG_CONFIG_HOME
  mkdir -p "$HOME/.config/xfce4/xfconf/xfce-perchannel-xml" "$TMPDIR" "$HOME/.claude/sessions" "$W/fake"
  cp "$HERE/xfce4-panel.xml" "$HOME/.config/xfce4/xfconf/xfce-perchannel-xml/xfce4-panel.xml"
  Xvfb :77 -screen 0 900x420x24 >/dev/null 2>&1 & XVFB=$!
  trap 'kill $XVFB 2>/dev/null || true; pkill -f "$W/fake/" 2>/dev/null || true; rm -rf "$W"' EXIT
  sleep 1
  export DISPLAY=:77 W
  dbus-run-session -- "$0" --inner "$EXE" "$FAKE" "$OUT"
  exit 0
fi

# --- inside the private session bus ----------------------------------------------
cp "$FAKE" "$W/fake/claude"
session() { # name status [waitingFor]
  "$W/fake/claude" >/dev/null 2>&1 & local pid=$!
  local extra=""; [ -n "${3:-}" ] && extra=", \"waitingFor\": \"$3\""
  printf '{"pid": %s, "name": "%s", "status": "%s", "kind": "interactive"%s}\n' "$pid" "$1" "$2" "$extra" > "$HOME/.claude/sessions/$pid.json"
  echo "$pid"
}
xsetroot -solid '#5e81ac' 2>/dev/null || true
xfce4-panel >/dev/null 2>&1 &
sleep 3
WAIT=$(session docs-site waiting "permission prompt")
session api-refactor busy >/dev/null
session notes idle >/dev/null
"$EXE" >/dev/null 2>&1 &
sleep 6

shot() { import -window root "$OUT/$1-full.png"; convert "$OUT/$1-full.png" -crop "$2" +repage "$OUT/$1.png"; rm "$OUT/$1-full.png"; }
shot panel-red 300x32+600+388

# Tooltip: hover the dot (systray sits left of the clock at the right end;
# 900x420 screen, 32 px panel at the bottom: the dot is at about 853,404).
xdotool mousemove 853 404; sleep 3
shot tooltip 340x112+560+308
# Menu: click the dot.
xdotool click 1; sleep 2
shot menu 545x270+355+150
xdotool key Escape; sleep 1

# Orange, then green.
printf '{"pid": %s, "name": "docs-site", "status": "busy", "kind": "interactive"}\n' "$WAIT" > "$HOME/.claude/sessions/$WAIT.json"
sleep 3; shot panel-orange 300x32+600+388
for f in "$HOME"/.claude/sessions/*.json; do sed -i 's/"busy"/"idle"/' "$f"; done
sleep 3; shot panel-green 300x32+600+388
echo "screenshots in $OUT"
