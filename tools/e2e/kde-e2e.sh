#!/usr/bin/env bash
# End-to-end test of the tray in a real KDE Plasma panel (CI runners).
#
# KWin runs on a virtual screen (no real display) with plasmashell and its
# default panel; kded provides the StatusNotifierWatcher, as in a normal
# Plasma session. A fake "claude" process and a session file that waits for a
# permission make the dot red. Checked:
#  - Plasma's StatusNotifierWatcher lists our tray;
#  - a screenshot taken with Spectacle shows the dot's red (saved for review).
# Everything lives in a scratch HOME; nothing of the real user is read.
#
# Needs: kwin-wayland, plasma-desktop, spectacle, dbus-run-session, python3
# with Pillow.
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
  unset XDG_CONFIG_HOME WAYLAND_DISPLAY DISPLAY
  plasmashell --version 2>/dev/null || true
  kwin_wayland --version 2>/dev/null || true
  dbus-run-session -- "$0" --inner "$EXE" "$FAKE"
  exit 0
fi

# --- inside the private session bus --------------------------------------------------
export XDG_SESSION_TYPE=wayland XDG_CURRENT_DESKTOP=KDE KDE_FULL_SESSION=true KDE_SESSION_VERSION=6
mkdir -p "$HOME/.claude/sessions"
cp "$FAKE" "$OUT/fake/claude"

PIDS=()
cleanup() { kill "${PIDS[@]}" 2>/dev/null || true; }
trap cleanup EXIT
show_logs() {
  local f
  for f in kwin kded plasmashell spectacle; do
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

kwin_wayland --virtual --width 1280 --height 800 --no-lockscreen --socket wayland-agsem > "$OUT/kwin.log" 2>&1 &
PIDS+=($!)
wait_for '[ -S "$XDG_RUNTIME_DIR/wayland-agsem" ]' 'KWin runs on a virtual screen' 60
export WAYLAND_DISPLAY=wayland-agsem QT_QPA_PLATFORM=wayland

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
PIDS+=($!)
wait_for 'has_owner org.kde.plasmashell' 'plasmashell runs' 60

shot() { # file: full-screen screenshot with Spectacle (KWin allows its screenshot interface to Spectacle)
  rm -f "$1"
  spectacle --background --nonotify --fullscreen --output "$1" >> "$OUT/spectacle.log" 2>&1 \
    || { show_logs; fail "screenshot with Spectacle ($1)"; }
  for _ in $(seq 20); do [ -s "$1" ] && return 0; sleep 0.5; done
  show_logs; fail "screenshot with Spectacle ($1)"
}
panel_reds() { # file: pixels in the dot's red in the bottom 80 px (the default panel), also saved as *-panel.png
  python3 - "$1" <<'PY'
import sys
from PIL import Image
im = Image.open(sys.argv[1]).convert("RGB")
w, h = im.size
panel = im.crop((0, h - 80, w, h))
panel.save(sys.argv[1].replace(".png", "-panel.png"))
px = panel.tobytes()
print(sum(1 for i in range(0, len(px), 3)
          if abs(px[i] - 220) < 30 and abs(px[i + 1] - 38) < 30 and abs(px[i + 2] - 38) < 30))
PY
}

# The panel before the tray starts, so red already on screen is not counted.
sleep 8
shot "$OUT/kde-before.png"
before=$(panel_reds "$OUT/kde-before.png")
echo "red pixels in the panel before the tray: $before"

"$OUT/fake/claude" & PIDS+=($!); P0=$!
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
reds=$(panel_reds "$OUT/kde-red.png")
echo "red pixels in the panel with the tray: $reds"
[ "$((reds - before))" -ge 20 ] || { show_logs; fail 'the dot in the Plasma panel is red'; }
ok 'the dot in the Plasma panel is red'

kill -0 "$TRAY" 2>/dev/null || { show_logs; fail 'tray app keeps running'; }
ok 'tray app keeps running'
echo 'All KDE Plasma checks passed.'
