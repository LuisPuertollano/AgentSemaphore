#!/usr/bin/env bash
# End-to-end test of the tray in a real KDE Plasma panel (CI runners).
#
# A Plasma X11 session on a virtual X display (Xvfb): kwin_x11, kded with its
# StatusNotifierWatcher module, and plasmashell with its default panel. A fake
# "claude" process and a session file that waits for a permission make the
# dot red. Checked:
#  - Plasma has a panel with the system tray in it;
#  - Plasma's StatusNotifierWatcher lists our tray;
#  - a screenshot of the display gained the dot's red, compared with one
#    taken before the tray started (both saved for review).
# Why X11 and not Wayland: on a runner without a GPU, KWin 6 on Wayland
# renders in software and cancels every screenshot request (FINDINGS). The
# panel, the tray and the protocol between them are the same in both.
# Everything lives in a scratch HOME; nothing of the real user is read.
#
# Needs: xvfb, kwin-x11, plasma-desktop, libxcb-cursor0, imagemagick,
# x11-utils, dbus-run-session, python3 with Pillow.
#
# usage: kde-e2e.sh <agentsemaphore binary> <fakeagent binary>
set -euo pipefail
if [ "${1:-}" = --inner ]; then INNER=1; shift; else INNER=0; fi
EXE=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
FAKE=$(cd "$(dirname "$2")" && pwd)/$(basename "$2")
OUT=${E2E_OUT:-${RUNNER_TEMP:-/tmp}/e2e-kde}

ok()   { echo "ok  $*"; }
fail() { echo "FAILED: $*" >&2; exit 1; }

if [ "$INNER" = 0 ]; then
  [ -n "${CI:-}" ] || fail "refusing to run outside CI (set CI=1 to run it on purpose; it only touches $OUT)"
  rm -rf "$OUT"; mkdir -p "$OUT/home" "$OUT/tmp" "$OUT/fake" "$OUT/rt"
  chmod 700 "$OUT/rt"
  export HOME="$OUT/home" TMPDIR="$OUT/tmp" XDG_RUNTIME_DIR="$OUT/rt" E2E_OUT="$OUT"
  export LANG=en_US.UTF-8 LANGUAGE=en
  unset XDG_CONFIG_HOME WAYLAND_DISPLAY
  kwin_x11 --version 2>/dev/null || true
  Xvfb :88 -screen 0 1280x800x24 > "$OUT/xvfb.log" 2>&1 & XVFB=$!
  trap 'kill $XVFB 2>/dev/null || true' EXIT
  for _ in $(seq 40); do [ -e /tmp/.X11-unix/X88 ] && break; sleep 0.25; done
  export DISPLAY=:88
  dbus-run-session -- "$0" --inner "$EXE" "$FAKE"
  exit 0
fi

# --- inside the private session bus --------------------------------------------------
export XDG_SESSION_TYPE=x11 XDG_CURRENT_DESKTOP=KDE KDE_FULL_SESSION=true KDE_SESSION_VERSION=6
export QT_QPA_PLATFORM=xcb
# KDE's menu is plasma-applications.menu; without the prefix KService finds no
# .desktop files ("applications.menu not found").
export XDG_MENU_PREFIX=plasma-
mkdir -p "$HOME/.claude/sessions"
cp "$FAKE" "$OUT/fake/claude"

PIDS=()
cleanup() { kill "${PIDS[@]}" 2>/dev/null || true; }
trap cleanup EXIT
show_logs() {
  local f
  for f in kwin kded plasmashell kbuildsycoca; do
    [ -f "$OUT/$f.log" ] && { echo "--- $f log (last 40 lines)"; tail -40 "$OUT/$f.log"; }
  done
  echo "--- tray output"; cat "$OUT/tray.out" 2>/dev/null || true
}
wait_for() { # command, description, timeout seconds
  local i
  for ((i = 0; i < $3 * 2; i++)); do
    if eval "$1" >/dev/null 2>&1; then ok "$2"; return 0; fi
    sleep 0.5
  done
  show_logs; fail "$2"
}
has_owner() { gdbus call --session --dest org.freedesktop.DBus --object-path /org/freedesktop/DBus \
  --method org.freedesktop.DBus.NameHasOwner "$1" | grep -q true; }

# Programs that D-Bus starts on demand (kactivitymanagerd, which plasmashell
# needs) must find the display too.
gdbus call --session --dest org.freedesktop.DBus --object-path /org/freedesktop/DBus \
  --method org.freedesktop.DBus.UpdateActivationEnvironment \
  "{'DISPLAY': '$DISPLAY', 'QT_QPA_PLATFORM': 'xcb', 'XDG_SESSION_TYPE': 'x11', 'XDG_CURRENT_DESKTOP': 'KDE', 'KDE_FULL_SESSION': 'true', 'KDE_SESSION_VERSION': '6', 'XDG_MENU_PREFIX': 'plasma-', 'XDG_RUNTIME_DIR': '$XDG_RUNTIME_DIR', 'HOME': '$HOME'}" >/dev/null
command -v kbuildsycoca6 >/dev/null && timeout 60 kbuildsycoca6 > "$OUT/kbuildsycoca.log" 2>&1 || true

xdpyinfo -display "$DISPLAY" >/dev/null 2>&1 || fail 'the virtual X display (Xvfb) runs'
ok 'the virtual X display (Xvfb) runs'
kwin_x11 > "$OUT/kwin.log" 2>&1 &
PIDS+=($!)
wait_for 'xprop -root _NET_SUPPORTING_WM_CHECK | grep -q "window id"' 'kwin_x11 manages the display' 60

KDED=$(command -v kded6 || command -v kded5 || ls /usr/lib/*/libexec/kf6/kded6 2>/dev/null | head -1 || true)
[ -n "$KDED" ] || fail 'kded is installed'
"$KDED" > "$OUT/kded.log" 2>&1 &
PIDS+=($!)
KDEDNAME=org.kde.kded6; [[ "$KDED" == *kded5 ]] && KDEDNAME=org.kde.kded5
wait_for "has_owner $KDEDNAME" 'kded runs' 30
gdbus call --session --dest "$KDEDNAME" --object-path /kded --method "$KDEDNAME.loadModule" statusnotifierwatcher \
  > "$OUT/kded-load.txt" 2>&1 || true
wait_for 'has_owner org.kde.StatusNotifierWatcher' "Plasma's StatusNotifierWatcher is on the bus" 30

plasmashell --no-respawn > "$OUT/plasmashell.log" 2>&1 &
PLASMA=$!
PIDS+=($PLASMA)
wait_for 'has_owner org.kde.plasmashell' 'plasmashell runs' 60
plasma_script() { gdbus call --session --dest org.kde.plasmashell --object-path /PlasmaShell \
  --method org.kde.PlasmaShell.evaluateScript "$1"; }
tray_in_panel='print(panels().filter(p => p.widgets("org.kde.plasma.systemtray").length > 0).length)'
wait_for "kill -0 $PLASMA && plasma_script '$tray_in_panel' | grep -q \"'[1-9]\"" 'Plasma has a panel with the system tray' 60
plasma_script 'print(panels().map(p => p.location + ":" + p.widgets().map(w => w.type).join(",")).join(" | "))' \
  | tee "$OUT/panels.txt"

shot() { # file: the whole virtual display, straight from the X server
  timeout 30 import -display "$DISPLAY" -window root "$1" || { show_logs; fail "screenshot ($1)"; }
  [ -s "$1" ] || { show_logs; fail "screenshot ($1)"; }
}
reds() { # file: pixels in the dot's red on the screen; the bottom 80 px (default panel) also saved as *-panel.png
  /usr/bin/python3 - "$1" <<'PY'
import sys
from PIL import Image
im = Image.open(sys.argv[1]).convert("RGB")
w, h = im.size
im.crop((0, h - 80, w, h)).save(sys.argv[1].replace(".png", "-panel.png"))  # for review
px = im.tobytes()  # the whole screen: the before/after difference cancels the wallpaper
print(sum(1 for i in range(0, len(px), 3)
          if abs(px[i] - 220) < 30 and abs(px[i + 1] - 38) < 30 and abs(px[i + 2] - 38) < 30))
PY
}

# The display before the tray starts, so red already on screen is not counted.
sleep 8
shot "$OUT/kde-before.png"
before=$(reds "$OUT/kde-before.png")
echo "red pixels before the tray: $before"

"$OUT/fake/claude" & P0=$!; PIDS+=($P0)
printf '{"pid": %s, "name": "e2e-wait", "status": "waiting", "kind": "interactive", "waitingFor": "permission prompt"}\n' \
  "$P0" > "$HOME/.claude/sessions/$P0.json"
"$EXE" > "$OUT/tray.out" 2>&1 & TRAY=$!; PIDS+=($TRAY)

registered() { gdbus call --session --dest org.kde.StatusNotifierWatcher --object-path /StatusNotifierWatcher \
  --method org.freedesktop.DBus.Properties.Get org.kde.StatusNotifierWatcher RegisteredStatusNotifierItems; }
wait_for 'registered | grep -q "/"' "Plasma's watcher lists our tray" 20
registered | tee "$OUT/registered.txt"

sleep 8
shot "$OUT/kde-red.png"
ok 'screenshot saved'
after=$(reds "$OUT/kde-red.png")
echo "red pixels with the tray: $after"
[ "$((after - before))" -ge 20 ] || { show_logs; fail 'the dot in the Plasma panel is red'; }
ok 'the dot in the Plasma panel is red'

kill -0 "$TRAY" 2>/dev/null || { show_logs; fail 'tray app keeps running'; }
ok 'tray app keeps running'
echo 'All KDE Plasma checks passed.'
