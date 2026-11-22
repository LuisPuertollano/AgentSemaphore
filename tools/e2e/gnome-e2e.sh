#!/usr/bin/env bash
# End-to-end test of the tray in a real GNOME Shell panel (CI runners).
#
# GNOME Shell runs headless (a virtual monitor, no real screen) with Ubuntu's
# AppIndicator extension, the one that makes tray icons appear in GNOME. A
# fake "claude" process and a session file that waits for a permission make
# the dot red. Checked:
#  - the extension's StatusNotifierWatcher lists our tray;
#  - GNOME's top bar holds a visible AppIndicator button for it (asked inside
#    the shell through org.gnome.Shell.Eval);
#  - a screenshot of the top bar shows the dot's red (saved for review too).
# To ask the shell and take the screenshot, a tiny test-only extension turns
# on GNOME's "unsafe mode" (the same switch as Looking Glass). It lives only in
# the scratch HOME below; nothing of the real user is read or written.
#
# Needs: gnome-shell, gnome-shell-extension-appindicator, dbus-run-session,
# python3 with Pillow. A system bus with logind,
# as GitHub's Ubuntu runners have.
#
# usage: gnome-e2e.sh <agentsemaphore binary> <fakeagent binary>
set -euo pipefail
if [ "${1:-}" = --inner ]; then INNER=1; shift; else INNER=0; fi
EXE=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
FAKE=$(cd "$(dirname "$2")" && pwd)/$(basename "$2")
OUT=${E2E_OUT:-${RUNNER_TEMP:-/tmp}/e2e-gnome}

ok()   { echo "ok  $*"; }
fail() { echo "FAILED: $*" >&2; exit 1; }

if [ "$INNER" = 0 ]; then
  [ -n "${CI:-}" ] || fail "refusing to run outside CI (set CI=1 to run it on purpose; it only touches $OUT)"
  rm -rf "$OUT"; mkdir -p "$OUT/home" "$OUT/tmp" "$OUT/fake" "$OUT/rt"
  chmod 700 "$OUT/rt"
  export HOME="$OUT/home" TMPDIR="$OUT/tmp" XDG_RUNTIME_DIR="$OUT/rt" E2E_OUT="$OUT"
  export LANG=en_US.UTF-8 LANGUAGE=en GSETTINGS_BACKEND=keyfile
  unset XDG_CONFIG_HOME WAYLAND_DISPLAY DISPLAY
  gnome-shell --version
  dbus-run-session -- "$0" --inner "$EXE" "$FAKE"
  exit 0
fi

# --- inside the private session bus --------------------------------------------------
UNSAFE=agentsemaphore-test-unsafe-mode@example.invalid
mkdir -p "$HOME/.local/share/gnome-shell/extensions/$UNSAFE" "$HOME/.claude/sessions"
major=$(gnome-shell --version | grep -o '[0-9]\+' | head -1)
cat > "$HOME/.local/share/gnome-shell/extensions/$UNSAFE/metadata.json" <<EOF
{ "uuid": "$UNSAFE", "name": "Test only: unsafe mode", "description": "CI only", "shell-version": ["$major"] }
EOF
cat > "$HOME/.local/share/gnome-shell/extensions/$UNSAFE/extension.js" <<'EOF'
import {Extension} from 'resource:///org/gnome/shell/extensions/extension.js';
export default class TestUnsafeMode extends Extension {
    enable() { global.context.unsafe_mode = true; }
    disable() { global.context.unsafe_mode = false; }
}
EOF
gsettings set org.gnome.shell enabled-extensions "['ubuntu-appindicators@ubuntu.com', '$UNSAFE']"
gsettings set org.gnome.shell disable-user-extensions false
gsettings set org.gnome.shell disable-extension-version-validation true

args=(--headless --wayland --virtual-monitor 1280x800)
gnome-shell --help 2>&1 | grep -q -- '--no-x11' && args+=(--no-x11)
gnome-shell "${args[@]}" > "$OUT/gnome-shell.log" 2>&1 &
SHELLPID=$!
cp "$FAKE" "$OUT/fake/claude"
cleanup() { kill "${TRAY:-}" "${P0:-}" "$SHELLPID" 2>/dev/null || true; }
trap cleanup EXIT

show_logs() {
  echo "--- gnome-shell log (last 60 lines)"; tail -60 "$OUT/gnome-shell.log" || true
  echo "--- tray output"; cat "$OUT/tray.out" 2>/dev/null || true
}
wait_for() { # command, description, timeout seconds
  local i
  for ((i = 0; i < $3 * 2; i++)); do
    if eval "$1" >/dev/null 2>&1; then ok "$2"; return 0; fi
    kill -0 "$SHELLPID" 2>/dev/null || { show_logs; fail "gnome-shell exited ($2)"; }
    sleep 0.5
  done
  show_logs; fail "$2"
}
has_owner() { gdbus call --session --dest org.freedesktop.DBus --object-path /org/freedesktop/DBus \
  --method org.freedesktop.DBus.NameHasOwner "$1" | grep -q true; }
shell_eval() { gdbus call --session --dest org.gnome.Shell --object-path /org/gnome/Shell \
  --method org.gnome.Shell.Eval "$1"; }

wait_for 'has_owner org.gnome.Shell' 'GNOME Shell runs headless' 60
wait_for 'has_owner org.kde.StatusNotifierWatcher' "the AppIndicator extension's watcher is on the bus" 30
wait_for 'shell_eval 1 | grep -q "(true,"' 'test extension turned on unsafe mode (Eval answers)' 20
shell_eval 'Main.extensionManager.getUuids().map(u => u + ":" + Main.extensionManager.lookup(u)?.state).join(" ")' \
  | tee "$OUT/extensions.txt"

# A session that waits for a permission, then the tray.
"$OUT/fake/claude" & P0=$!
printf '{"pid": %s, "name": "e2e-wait", "status": "waiting", "kind": "interactive", "waitingFor": "permission prompt"}\n' \
  "$P0" > "$HOME/.claude/sessions/$P0.json"
"$EXE" > "$OUT/tray.out" 2>&1 & TRAY=$!

registered() { gdbus call --session --dest org.kde.StatusNotifierWatcher --object-path /StatusNotifierWatcher \
  --method org.freedesktop.DBus.Properties.Get org.kde.StatusNotifierWatcher RegisteredStatusNotifierItems; }
wait_for 'registered | grep -q "/"' "GNOME's watcher lists our tray" 20
registered | tee "$OUT/registered.txt"

panel='Object.entries(Main.panel.statusArea).filter(([k, v]) => k.startsWith("appindicator-") && v)
  .map(([k, v]) => k + " visible=" + v.visible + " mapped=" + v.mapped).join("; ")'
wait_for "shell_eval '$panel' | grep -q 'visible=true mapped=true'" 'GNOME top bar shows a visible AppIndicator button' 20
shell_eval "$panel" | tee "$OUT/panel.txt"

# Screenshot of the whole virtual monitor; the top bar is its first ~32 px.
sleep 3
gdbus call --session --dest org.gnome.Shell.Screenshot --object-path /org/gnome/Shell/Screenshot \
  --method org.gnome.Shell.Screenshot.Screenshot false false "$OUT/gnome-red.png" > "$OUT/screenshot.txt" 2>&1 \
  || { cat "$OUT/screenshot.txt"; show_logs; fail 'screenshot of the virtual monitor'; }
[ -s "$OUT/gnome-red.png" ] || fail 'screenshot of the virtual monitor'
ok 'screenshot saved'
reds=$(python3 - "$OUT/gnome-red.png" <<'PY'
import sys
from PIL import Image
im = Image.open(sys.argv[1]).convert("RGB")
top = im.crop((0, 0, im.size[0], 40))
top.save(sys.argv[1].replace(".png", "-topbar.png"))
px = top.tobytes()
print(sum(1 for i in range(0, len(px), 3)
          if abs(px[i] - 220) < 30 and abs(px[i + 1] - 38) < 30 and abs(px[i + 2] - 38) < 30))
PY
)
echo "red pixels in the top bar: $reds"
[ "$reds" -ge 20 ] || fail 'the dot in the top bar is red'
ok 'the dot in the top bar is red'

kill -0 "$TRAY" 2>/dev/null || { show_logs; fail 'tray app keeps running'; }
ok 'tray app keeps running'
echo 'All GNOME checks passed.'
