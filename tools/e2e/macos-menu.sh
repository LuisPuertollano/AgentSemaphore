#!/usr/bin/env bash
# macOS (CI): click the tray's menu bar item through the Accessibility API
# (System Events) and check that its menu opens. Run by unix-e2e.sh while the
# tray runs with a session that waits for a permission ("e2e-wait").
#
# In fyne.io/systray every click on the item reaches the left-click handler.
# Before v0.1.0-alpha.6 that handler opened Claude, so the menu never opened
# (FINDINGS, "macOS: clicks and the Dock"). Here the click must open the menu.
# GitHub's macOS runners allow this: measured 2026-11-25 on macOS Apple
# silicon and Intel, where the item is "status menu" in menu bar 1.
#
# usage: macos-menu.sh <out dir>
set -uo pipefail
OUT=$1
PROC='tell application "System Events" to tell process "agentsemaphore"'
as() { osascript -e "$1" 2>&1; }
fail() { echo "FAILED: $*" >&2; exit 1; }

desc=$(as "$PROC to get description of menu bar item 1 of menu bar 1")
[ "$desc" = "status menu" ] || fail "the tray is a status item in the menu bar (System Events says: $desc)"
echo "ok  the tray is a status item in the menu bar"

as "$PROC to click menu bar item 1 of menu bar 1" >/dev/null
sleep 1.5
items=$(as "$PROC to get name of every menu item of menu 1 of menu bar item 1 of menu bar 1")
printf '%s\n' "$items" > "$OUT/macos-menu.txt"
screencapture -x "$OUT/macos-menu.png" 2>/dev/null || true
as 'tell application "System Events" to key code 53' >/dev/null   # Escape closes the menu

case "$items" in
  *"Open Claude"*"Quit"*) echo "ok  a click opens the menu (Open Claude … Quit)" ;;
  *) echo "menu items: $items"; fail 'a click opens the menu' ;;
esac
case "$items" in
  *"e2e-wait"*"permission prompt"*) echo "ok  the opened menu lists the waiting session and why" ;;
  *) echo "menu items: $items"; fail 'the opened menu lists the waiting session' ;;
esac
