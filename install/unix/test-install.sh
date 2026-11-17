#!/usr/bin/env bash
# End-to-end test of install.sh on Linux and macOS (CI). HOME points into a
# scratch folder, so the real user's files and autostart are never touched.
#
# usage: test-install.sh <locally built agentsemaphore> [release tag to download, e.g. v0.1.0-alpha.1]
set -euo pipefail
[ -n "${CI:-}" ] || { echo "refusing to run outside CI (set CI=1 to run it on purpose)"; exit 1; }

HERE=$(cd "$(dirname "$0")" && pwd)
INSTALL="$HERE/install.sh"
BUILT=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
TAG=${2:-}
WORK=$(mktemp -d)
export HOME="$WORK/home"
unset XDG_CONFIG_HOME DISPLAY WAYLAND_DISPLAY
mkdir -p "$HOME"
DEST="$HOME/.local/bin/agentsemaphore"
if [ "$(uname -s)" = Darwin ]; then
  AUTOFILE="$HOME/Library/LaunchAgents/io.github.luispuertollano.agentsemaphore.plist"
else
  AUTOFILE="$HOME/.config/autostart/agentsemaphore.desktop"
fi

ok()   { echo "ok  $*"; }
fail() { echo "FAILED: $*" >&2; exit 1; }
running() { pgrep -f -x "$DEST" >/dev/null 2>&1; }
cleanup() { pkill -f -x "$DEST" 2>/dev/null || true; rm -rf "$WORK"; }
trap cleanup EXIT

# 1. --dry-run changes nothing.
bash "$INSTALL" --exe "$BUILT" --autostart --dry-run > "$WORK/dry.txt"
[ ! -e "$DEST" ] || fail 'dry run: no binary'
[ ! -e "$AUTOFILE" ] || fail 'dry run: no autostart entry'
grep -q 'would run' "$WORK/dry.txt" || fail 'dry run lists the changes'
ok 'dry run changes nothing and lists the changes'

# 2. Install a local build with autostart.
bash "$INSTALL" --exe "$BUILT" --autostart --no-start
[ -x "$DEST" ] || fail 'binary installed'
"$DEST" -version | grep -q '^agentsemaphore ' || fail 'installed binary runs'
ok 'binary installed and runs'
grep -qF "$DEST" "$AUTOFILE" || fail 'autostart entry points to the installed binary'
ok 'autostart entry points to the installed binary'

# 3. Re-install while it runs: the running copy is stopped first.
"$DEST" > /dev/null 2>&1 &
sleep 2
running || fail 'installed copy runs'
bash "$INSTALL" --exe "$BUILT" --autostart --no-start
sleep 0.5
running && fail 're-install stops the running copy'
ok 're-install stops the running copy'

# 4. Uninstall: process stopped, binary and our autostart entry gone, settings kept.
"$DEST" > /dev/null 2>&1 &
sleep 2
bash "$INSTALL" --uninstall
running && fail 'uninstall stops the app'
[ ! -e "$DEST" ] || fail 'uninstall removes the binary'
[ ! -e "$AUTOFILE" ] || fail 'uninstall removes our autostart entry'
ok 'uninstall stops the app, removes the binary and the autostart entry'

# 5. An autostart entry that starts something else is left alone.
mkdir -p "$(dirname "$AUTOFILE")"
echo 'Exec="/somewhere/else/agentsemaphore"' > "$AUTOFILE"
bash "$INSTALL" --exe "$BUILT" --no-start | tee "$WORK/out.txt"
grep -q 'another AgentSemaphore starts at sign-in' "$WORK/out.txt" || fail 'warns about a foreign autostart entry'
bash "$INSTALL" --uninstall
grep -q '/somewhere/else' "$AUTOFILE" || fail 'uninstall leaves a foreign autostart entry alone'
ok 'a foreign autostart entry is reported and left alone'
rm -f "$AUTOFILE"

# 6. Download a published release (checks the SHA-256), then the newest one.
if [ -n "$TAG" ]; then
  bash "$INSTALL" --version "$TAG" --no-start | tee "$WORK/out.txt"
  grep -q 'SHA-256 checked' "$WORK/out.txt" || fail 'download is checked'
  [ "$("$DEST" -version)" = "agentsemaphore $TAG" ] || fail "installed $TAG"
  ok "release $TAG downloaded, checked and installed"
  bash "$INSTALL" --no-start | tee "$WORK/out.txt"
  grep -q 'SHA-256 checked' "$WORK/out.txt" || fail 'newest release: download is checked'
  ok "newest release downloaded and checked: $("$DEST" -version)"
  # Same without the API (rate limits): found on the releases page instead.
  AGENTSEMAPHORE_RELEASES_API="https://api.github.com/repos/LuisPuertollano/AgentSemaphore/no-such-endpoint" \
    bash "$INSTALL" --no-start 2>&1 | tee "$WORK/out.txt"
  grep -q 'on the releases page' "$WORK/out.txt" || fail 'newest release found without the API'
  grep -q 'SHA-256 checked' "$WORK/out.txt" || fail 'fallback: download is checked'
  ok 'newest release found on the releases page when the API does not answer'
  bash "$INSTALL" --uninstall
fi

echo 'All installer checks passed.'
