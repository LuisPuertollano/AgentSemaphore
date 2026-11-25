#!/usr/bin/env bash
# macOS (CI): click the tray's menu bar item through the Accessibility API and
# read the menu that opens. Run by unix-e2e.sh while the tray runs.
#
# FIRST VERSION, DIAGNOSTIC: prints what System Events sees, so we learn
# whether GitHub's macOS runners let a script click the menu bar. It does not
# fail the job yet.
#
# usage: macos-menu.sh <out dir>
set -uo pipefail
OUT=$1
as() { osascript -e "$1" 2>&1; }

echo "--- System Events: processes named like ours"
as 'tell application "System Events" to get {name, background only} of (every process whose name contains "agentsemaphore")'

echo "--- menu bars of the process"
as 'tell application "System Events" to tell process "agentsemaphore" to get count of menu bars'
for mb in 1 2; do
  echo "--- items of menu bar $mb"
  as "tell application \"System Events\" to tell process \"agentsemaphore\" to get {name, description, title} of every menu bar item of menu bar $mb"
done

for mb in 2 1; do
  echo "--- click menu bar item 1 of menu bar $mb"
  as "tell application \"System Events\" to tell process \"agentsemaphore\" to click menu bar item 1 of menu bar $mb"
  sleep 1.5
  echo "--- menu items after the click (menu bar $mb)"
  as "tell application \"System Events\" to tell process \"agentsemaphore\" to get name of every menu item of menu 1 of menu bar item 1 of menu bar $mb" \
    | tee "$OUT/macos-menu-$mb.txt"
  screencapture -x "$OUT/macos-menu-$mb.png" 2>/dev/null || true
  as 'tell application "System Events" to key code 53' >/dev/null   # Escape closes the menu
  sleep 0.5
done
exit 0
