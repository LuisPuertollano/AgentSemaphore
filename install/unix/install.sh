#!/usr/bin/env bash
# Installs AgentSemaphore for the current user on Linux or macOS (no sudo).
#
#   curl -fsSL https://raw.githubusercontent.com/LuisPuertollano/AgentSemaphore/main/install/unix/install.sh | bash -s -- --autostart
#
# What it does:
#   - downloads the newest release for this OS and CPU (pre-releases included),
#     or --version <tag>, or uses --exe <file> you already have;
#   - checks the download against the release's SHA256SUMS.txt;
#   - installs it as ~/.local/bin/agentsemaphore (--dir to change);
#   - --autostart: starts it at sign-in, with the same entry the app's own
#     "Start with the system" menu uses (Linux: ~/.config/autostart, macOS:
#     ~/Library/LaunchAgents);
#   - starts it, unless --no-start or there is no graphical session.
# --uninstall stops it and removes the binary and our autostart entry; your
# settings and log stay in the app's config folder (the script says where).
# --dry-run prints every change without making it.
set -euo pipefail

REPO="LuisPuertollano/AgentSemaphore"
DIR="$HOME/.local/bin"
EXE="" VERSION="" AUTOSTART=0 NOSTART=0 UNINSTALL=0 DRY=0

usage() {
  cat <<'USAGE'
usage: install.sh [--exe FILE | --version TAG] [--dir DIR] [--autostart] [--no-start] [--dry-run]
       install.sh --uninstall [--dir DIR] [--dry-run]
  default: download the newest release (pre-releases included) and check its SHA-256
USAGE
}
die() { echo "error: $*" >&2; exit 1; }
say() { echo "$*"; }
run() { if [ "$DRY" = 1 ]; then echo "would run: $*"; else "$@"; fi; }

while [ $# -gt 0 ]; do
  case "$1" in
    --exe) EXE=${2:?--exe needs a file}; shift 2 ;;
    --version) VERSION=${2:?--version needs a tag}; shift 2 ;;
    --dir) DIR=${2:?--dir needs a folder}; shift 2 ;;
    --autostart) AUTOSTART=1; shift ;;
    --no-start) NOSTART=1; shift ;;
    --uninstall) UNINSTALL=1; shift ;;
    --dry-run) DRY=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown option $1 (see --help)" ;;
  esac
done
[ -n "$EXE" ] && [ -n "$VERSION" ] && die "--exe and --version do not go together"
if [ -n "$VERSION" ] && ! [[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$ ]]; then
  die "not a release tag: $VERSION (expected e.g. v0.1.0-alpha.1)"
fi

case "$(uname -s)" in
  Linux) OS=linux ;;
  Darwin) OS=darwin ;;
  *) die "unsupported system $(uname -s); on Windows use install/windows/install.ps1" ;;
esac
case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) die "unsupported CPU $(uname -m)" ;;
esac
DEST="$DIR/agentsemaphore"

if [ "$OS" = darwin ]; then
  AUTOFILE="$HOME/Library/LaunchAgents/io.github.luispuertollano.agentsemaphore.plist"
  CONFDIR="$HOME/Library/Application Support/AgentSemaphore"
else
  AUTOFILE="${XDG_CONFIG_HOME:-$HOME/.config}/autostart/agentsemaphore.desktop"
  CONFDIR="${XDG_CONFIG_HOME:-$HOME/.config}/AgentSemaphore"
fi

# Stops the copy running from $DEST (not other copies elsewhere).
stop_running() {
  local pids
  pids=$(pgrep -f -x "$DEST" 2>/dev/null || true)
  [ -z "$pids" ] && return 0
  say "Stopping the running copy (PID $pids)."
  run kill $pids
  [ "$DRY" = 1 ] && return 0
  for _ in 1 2 3 4 5 6 7 8 9 10; do pgrep -f -x "$DEST" >/dev/null 2>&1 || return 0; sleep 0.5; done
  kill -9 $pids 2>/dev/null || true
}

# True when the autostart entry starts our $DEST.
autostart_is_ours() { [ -f "$AUTOFILE" ] && grep -qF "$DEST" "$AUTOFILE"; }

sha256() { if command -v sha256sum >/dev/null; then sha256sum "$1"; else shasum -a 256 "$1"; fi | cut -d' ' -f1; }

# --- uninstall -----------------------------------------------------------------------
if [ "$UNINSTALL" = 1 ]; then
  [ -e "$DEST" ] || die "nothing installed at $DEST"
  stop_running
  if autostart_is_ours; then run "$DEST" -autostart off; fi
  run rm -f "$DEST"
  say "AgentSemaphore removed."
  [ -d "$CONFDIR" ] && say "Your settings and log are kept in $CONFDIR (delete the folder to remove them too)."
  exit 0
fi

# --- get the binary ---------------------------------------------------------------------
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
ASSET="agentsemaphore-$OS-$ARCH"
if [ -n "$EXE" ]; then
  [ -f "$EXE" ] || die "not found: $EXE"
  cp "$EXE" "$TMP/$ASSET"
  say "Installing $EXE (not downloaded, so no checksum to compare)."
else
  command -v curl >/dev/null || die "curl is needed to download the release"
  if [ -z "$VERSION" ]; then
    # GitHub's "latest" skips pre-releases, so ask the API for the newest one.
    VERSION=$(curl -fsSL -H 'Accept: application/vnd.github+json' "https://api.github.com/repos/$REPO/releases?per_page=1" |
      grep -m1 '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/') || true
    [ -n "$VERSION" ] || die "could not find a published release"
  fi
  base="https://github.com/$REPO/releases/download/$VERSION"
  say "Downloading $ASSET from $VERSION ..."
  curl -fsSL -o "$TMP/$ASSET" "$base/$ASSET" || die "download failed: $base/$ASSET"
  curl -fsSL -o "$TMP/SHA256SUMS.txt" "$base/SHA256SUMS.txt" || die "release $VERSION has no SHA256SUMS.txt; not installing an unchecked download"
  want=$(awk -v f="$ASSET" '$2 == f || $2 == "*" f { print $1 }' "$TMP/SHA256SUMS.txt")
  [ -n "$want" ] || die "SHA256SUMS.txt of $VERSION does not list $ASSET"
  got=$(sha256 "$TMP/$ASSET")
  [ "$got" = "$want" ] || die "SHA-256 mismatch for $ASSET (expected $want, got $got); not installed"
  say "SHA-256 checked."
fi
chmod 755 "$TMP/$ASSET"

# --- install -------------------------------------------------------------------------------
stop_running
run mkdir -p "$DIR"
run cp "$TMP/$ASSET" "$DEST.new"
run mv -f "$DEST.new" "$DEST"
if [ "$DRY" = 0 ]; then say "Installed: $("$DEST" -version) -> $DEST"; fi

if [ "$AUTOSTART" = 1 ]; then
  run "$DEST" -autostart on ||
    die "this build cannot set autostart itself (v0.1.0-alpha.1 and older); use \"Start with the system\" in its menu"
elif [ -f "$AUTOFILE" ] && ! autostart_is_ours; then
  say "note: another AgentSemaphore starts at sign-in ($AUTOFILE). Re-run with --autostart to point it here."
fi

if [ "$OS" = linux ]; then
  say "note: GNOME shows tray icons only with the AppIndicator extension (Ubuntu has it on by default)."
fi

if [ "$NOSTART" = 0 ]; then
  if [ "$OS" = linux ] && [ -z "${DISPLAY:-}${WAYLAND_DISPLAY:-}" ]; then
    say "No graphical session here, so not starting it now."
  else
    say "Starting AgentSemaphore."
    if [ "$DRY" = 1 ]; then echo "would run: $DEST &"; else nohup "$DEST" >/dev/null 2>&1 & fi
  fi
fi
case ":$PATH:" in *":$DIR:"*) ;; *) say "note: $DIR is not on your PATH (only needed to run it by hand)." ;; esac
say "To uninstall: bash install.sh --uninstall   (or the curl line with --uninstall)"
