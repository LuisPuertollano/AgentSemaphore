#!/usr/bin/env bash
# End-to-end test of the Go core on Linux and macOS (CI runners). No Claude
# installation is needed:
#  - fake "claude" processes (tools/e2e/fakeagent) and fake session files drive
#    the Claude Code source through every colour (-once);
#  - fake "copilot" and "codex" processes run our hook command as their agents
#    would, so the hook source and its agent-process lookup are exercised;
#  - the tray is started twice to check the single-instance lock;
#  - Linux: on a private session bus, a minimal StatusNotifierWatcher
#    (tools/e2e/snwatcher) reads back what a panel would show: the dot's
#    colour, the tooltip and the menu;
#  - macOS: a screenshot of the menu bar is saved for review (not an assertion).
# HOME, the config dir and TMPDIR point into a scratch folder, so nothing of
# the real user's ~/.claude is read or written.
#
# usage: unix-e2e.sh <agentsemaphore binary> [snwatcher binary (Linux)]
set -euo pipefail

if [ "${1:-}" = "--tray-linux" ]; then MODE=tray-linux; shift; else MODE=main; fi
EXE=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
WATCH=${2:-}
OUT=${E2E_OUT:-${RUNNER_TEMP:-/tmp}/e2e}

ok()   { echo "ok  $*"; }
fail() { echo "FAILED: $*" >&2; exit 1; }

if [ "$MODE" = main ]; then
  [ -n "${CI:-}" ] || fail "refusing to run outside CI (set CI=1 to run it on purpose; it only touches $OUT)"
  rm -rf "$OUT"; mkdir -p "$OUT/home" "$OUT/tmp" "$OUT/fake"
  # Built before HOME moves, so Go uses the real module and build caches.
  ROOT=$(cd "$(dirname "$0")/../.." && pwd)
  (cd "$ROOT" && go build -o "$OUT/fake/claude" ./tools/e2e/fakeagent) || fail 'build the fake agent process'
  cp "$OUT/fake/claude" "$OUT/fake/copilot"; cp "$OUT/fake/claude" "$OUT/fake/codex"
fi
export HOME="$OUT/home" TMPDIR="$OUT/tmp" E2E_OUT="$OUT"
unset XDG_CONFIG_HOME
SES="$HOME/.claude/sessions"
mkdir -p "$SES"

json() { python3 -c "import json,sys; d=json.load(sys.stdin); print($1)"; }
once() { "$EXE" -once; }
state() { once | json 'd["state"]'; }

write_session() { # pid name status [waitingFor]
  local extra=""
  [ -n "${4:-}" ] && extra=", \"waitingFor\": \"$4\""
  printf '{"pid": %s, "name": "%s", "status": "%s", "kind": "interactive"%s}\n' "$1" "$2" "$3" "$extra" > "$SES/$1.json"
}

logfile() { find "$HOME" -name agentsemaphore.log 2>/dev/null | head -1; }

# --- tray on Linux, inside dbus-run-session ------------------------------------------
if [ "$MODE" = tray-linux ]; then
  SNAP="$OUT/snap.json"
  "$WATCH" -out "$SNAP" > "$OUT/snwatcher.log" 2>&1 &
  wpid=$!
  sleep 1
  "$EXE" > "$OUT/tray.out" 2>&1 &
  tpid=$!
  trap 'kill $tpid $wpid 2>/dev/null || true' EXIT

  wait_snap() { # python condition on d, description, timeout seconds
    local i
    for ((i = 0; i < $3 * 2; i++)); do
      if [ -f "$SNAP" ] && python3 -c "import json,sys; d=json.load(open('$SNAP')); sys.exit(0 if ($1) else 1)" 2>/dev/null; then
        ok "$2"; return 0
      fi
      sleep 0.5
    done
    echo "--- last snapshot"; cat "$SNAP" 2>/dev/null || true
    echo "--- snwatcher log"; cat "$OUT/snwatcher.log" || true
    echo "--- tray output"; cat "$OUT/tray.out" || true
    fail "$2"
  }

  wait_snap 'd["registered"]' 'tray registered with the StatusNotifierWatcher' 15
  wait_snap 'd["state"] == "red"' 'tray dot is red' 20
  wait_snap '"1 waiting" in d["tooltip"] and "e2e-wait" in d["tooltip"]' 'tooltip counts the waiting session and names it' 10
  wait_snap 'any("e2e-wait" in m and "permission prompt" in m for m in d["menu"]) and d["menu"][-1] == "Quit"' 'menu shows the session, why it waits, and Quit' 10
  cp "$SNAP" "$OUT/snapshot-red.json"

  write_session "$P0" e2e-wait busy
  wait_snap 'd["state"] == "orange"' 'tray dot turns orange when the session works again' 20
  cp "$SNAP" "$OUT/snapshot-orange.json"

  kill -0 "$tpid" || fail 'tray app keeps running'
  ok 'tray app keeps running'
  exit 0
fi

# --- fake Claude Code sessions (-once) -------------------------------------------------
"$OUT/fake/claude" & P0=$!
"$OUT/fake/claude" & P1=$!
"$OUT/fake/claude" & P2=$!
cleanup() { kill "$P0" "$P1" "$P2" 2>/dev/null || true; pkill -f "$OUT/fake/" 2>/dev/null || true; [ -n "${TRAY:-}" ] && kill "$TRAY" 2>/dev/null || true; }
trap cleanup EXIT
sleep 0.5
for p in "$P0" "$P1" "$P2"; do kill -0 "$p" 2>/dev/null || fail "fake claude process $p is running"; done
ok 'fake claude processes are running'

[ "$(state)" = green ] || fail 'no sessions -> green'; ok 'no sessions -> green'
once | json '"\n".join(d["menu"])' | grep -q 'Claude Code' || fail 'menu names the Claude Code source'
ok 'menu names the Claude Code source'

write_session "$P0" e2e-wait waiting 'permission prompt'
write_session "$P1" e2e-busy busy
write_session "$P2" e2e-idle idle
[ "$(state)" = red ] || { once; fail 'waiting session -> red'; }; ok 'waiting session -> red'
[ "$(once | json 'len(d["sessions"])')" = 3 ] || { once; fail 'three live sessions seen (process name check works on this OS)'; }
ok 'three live sessions seen (process name check works on this OS)'
once | json '"\n".join(d["menu"])' | grep -q 'e2e-wait.*permission prompt' || fail 'menu shows the waiting session and why'
ok 'menu shows the waiting session and why'

write_session "$P0" e2e-wait idle
[ "$(state)" = orange ] || fail 'busy session -> orange'; ok 'busy session -> orange'

write_session "$P1" e2e-busy dreaming
[ "$(state)" = grey ] || fail 'unknown status -> grey'; ok 'unknown status -> grey'

kill "$P1"; wait "$P1" 2>/dev/null || true
[ "$(state)" = green ] || fail 'session of an exited process is ignored -> green'
ok 'session of an exited process is ignored -> green'

# Linux: procStart (start time in /proc/<pid>/stat, field 22) catches a reused pid.
if [ "$(uname -s)" = Linux ] && [ -r /etc/machine-id ]; then
  dom="linux:$(cat /etc/machine-id):$(readlink /proc/self/ns/pid)"
  ticks=$(sed 's/.*) //' "/proc/$P2/stat" | awk '{print $20}')
  printf '{"pid": %s, "name": "e2e-same", "status": "busy", "procStart": "%s", "pidDomain": "%s"}\n' "$P2" "$ticks" "$dom" > "$SES/$P2.json"
  [ "$(state)" = orange ] || { once; fail 'procStart matches the running process -> session shown'; }
  ok 'procStart matches the running process -> session shown'
  printf '{"pid": %s, "name": "e2e-reused", "status": "waiting", "procStart": "%s", "pidDomain": "%s"}\n' "$P2" "$((ticks + 1))" "$dom" > "$SES/$P2.json"
  [ "$(state)" = green ] || { once; fail 'procStart of an older process -> reused pid, session ignored'; }
  ok 'procStart of an older process -> reused pid, session ignored'
  write_session "$P2" e2e-idle idle
fi

# --- other agents through their hooks ------------------------------------------------------
hook() { # agent-binary agent event json: run our hook the way the agent would (via sh)
  "$OUT/fake/$1" sh -c 'printf "%s" "$3" | "$0" hook "$1" "$2"' "$EXE" "$2" "$3" "$4" &
}
menu() { once | json '"\n".join(d["menu"])'; }
hook copilot copilot notification '{"sessionId":"cp1","cwd":"/work/site","notification_type":"permission_prompt"}'; H1=$!
hook codex codex UserPromptSubmit '{"session_id":"cx1","cwd":"/work/api"}'; H2=$!
sleep 1
[ "$(state)" = red ] || { once; fail 'Copilot permission prompt (hook) -> red'; }
ok 'Copilot permission prompt (hook) -> red'
menu | grep -q 'Copilot .* site.*permission prompt' || { once; fail 'menu shows the Copilot session and why'; }
menu | grep -q 'Codex .* api' || { once; fail 'menu shows the Codex session'; }
ok 'menu shows the Copilot and Codex sessions under "Other agents"'
hook copilot copilot agentStop '{"sessionId":"cp1","cwd":"/work/site"}'; H3=$!
sleep 1
[ "$(state)" = orange ] || { once; fail 'Copilot turn done -> green, Codex still working -> orange'; }
ok 'Copilot turn done -> green, Codex still working -> orange'
kill "$H1" "$H3"; wait "$H1" "$H3" 2>/dev/null || true
menu | grep -q 'Copilot .* site' && { once; fail 'a session whose agent process exited is dropped'; }
ok 'a session whose agent process exited is dropped'
hook codex codex SessionEnd '{"session_id":"cx1","cwd":"/work/api"}'; H4=$!
sleep 1
[ "$(state)" = green ] || { once; fail 'Codex session end -> session removed -> green'; }
ok 'Codex session end -> session removed -> green'
kill "$H2" "$H4" 2>/dev/null || true
[ "$(printf '{"session_id":"g1"}' | "$EXE" hook gemini SessionStart)" = '{}' ] || fail 'Gemini hook prints {} (its documented no-op)'
ok 'Gemini hook prints {} (its documented no-op)'
[ "$(printf 'garbage' | "$EXE" hook codex Stop; echo "rc=$?")" = 'rc=0' ] || fail 'bad hook input: no output, exit 0'
ok 'bad hook input: no output, exit 0 (never in the way of the agent)'
"$EXE" hooks install copilot >/dev/null && [ -f "$HOME/.copilot/hooks/agentsemaphore.json" ] || fail 'hooks install copilot writes our own file'
[ "$("$EXE" hooks status copilot)" = installed ] || fail 'hooks status copilot'
"$EXE" hooks uninstall copilot >/dev/null && [ ! -e "$HOME/.copilot/hooks/agentsemaphore.json" ] || fail 'hooks uninstall copilot'
ok 'hooks install / status / uninstall copilot'

# --- tray: single instance --------------------------------------------------------------
write_session "$P0" e2e-wait waiting 'permission prompt'
case "$(uname -s)" in
  Linux)
    [ -n "$WATCH" ] || fail 'Linux needs the snwatcher binary as second argument'
    WATCH=$(cd "$(dirname "$WATCH")" && pwd)/$(basename "$WATCH")
    P0=$P0 dbus-run-session -- "$0" --tray-linux "$EXE" "$WATCH"
    ;;
  Darwin)
    "$EXE" > "$OUT/tray.out" 2>&1 & TRAY=$!
    sleep 4
    kill -0 "$TRAY" 2>/dev/null || { cat "$OUT/tray.out"; fail 'tray app keeps running'; }
    ok 'tray app keeps running'
    # Informational: does the bare binary get a Dock icon? ("Foreground" = yes, "UIElement" = no)
    lsappinfo list > "$OUT/lsappinfo.txt" 2>&1 || true
    grep -i -A12 'agentsemaphore' "$OUT/lsappinfo.txt" | grep -i -E 'agentsemaphore|type' | head -5 || echo 'note: not listed by lsappinfo'
    screencapture -x "$OUT/desktop.png" 2>/dev/null && ok 'screenshot saved (for review, not an assertion)' || echo 'note: no screenshot'
    ;;
esac

# Second copy while one runs (Linux: start one again outside the bus, it only needs the lock).
if [ -z "${TRAY:-}" ]; then "$EXE" > "$OUT/tray1.out" 2>&1 & TRAY=$!; sleep 2; fi
"$EXE" > "$OUT/tray2.out" 2>&1 & second=$!
for _ in $(seq 20); do kill -0 "$second" 2>/dev/null || break; sleep 0.5; done
kill -0 "$second" 2>/dev/null && fail 'second copy exits on its own (single instance)'
ok 'second copy exits on its own (single instance)'
grep -q 'already running' "$(logfile)" || fail 'second copy logged "already running"'
ok 'second copy logged "already running"'
cp "$(logfile)" "$OUT/agentsemaphore.log"

echo 'All end-to-end checks passed.'
