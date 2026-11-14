"""Read-only: dumps the Claude toasts stored in the Windows notification DB.

Works on a copy of the DB (never opens the original). Prints, oldest first, every
toast from a handler whose id mentions Claude/Anthropic in the last N hours: arrival
time, handler, the <text> lines, the launch attribute and the raw payload XML.

    python -I dump-toasts.py            # last 2 hours
    python -I dump-toasts.py 24 > toasts.txt

The output can contain conversation titles; check it before sharing it publicly.
"""
import datetime, os, re, shutil, sqlite3, sys, tempfile

hours = float(sys.argv[1]) if len(sys.argv) > 1 else 2
src = os.path.join(os.environ['LOCALAPPDATA'], 'Microsoft', 'Windows', 'Notifications')
tmp = tempfile.mkdtemp(prefix='as_dump_')
for n in ('wpndatabase.db', 'wpndatabase.db-wal'):
    if os.path.exists(os.path.join(src, n)):
        shutil.copyfile(os.path.join(src, n), os.path.join(tmp, n))
c = sqlite3.connect(os.path.join(tmp, 'wpndatabase.db'))
rows = c.execute(
    "select n.Id, n.ArrivalTime, n.Type, h.PrimaryId, n.Payload from Notification n "
    "join NotificationHandler h on h.RecordId = n.HandlerId "
    "where h.PrimaryId like '%Claude%' or h.PrimaryId like '%anthropic%' "
    "order by n.ArrivalTime").fetchall()
c.close()
shutil.rmtree(tmp, ignore_errors=True)

since = datetime.datetime.now(datetime.timezone.utc).replace(tzinfo=None) - datetime.timedelta(hours=hours)
shown = 0
for nid, arr, kind, handler, payload in rows:
    t = datetime.datetime(1601, 1, 1) + datetime.timedelta(microseconds=arr / 10)
    if t < since:
        continue
    p = payload.decode('utf-8', 'replace') if isinstance(payload, bytes) else str(payload)
    texts = [re.sub(r'<[^>]+>', '', x).strip() for x in re.findall(r'<text[^>]*>(.*?)</text>', p, re.S)]
    launch = re.search(r'\blaunch="([^"]*)"', p)
    local = t.replace(tzinfo=datetime.timezone.utc).astimezone()
    print('=' * 70)
    print('id=%s  %s  type=%s  handler=%s' % (nid, local.strftime('%Y-%m-%d %H:%M:%S'), kind, handler))
    for x in texts:
        print('  text:   %s' % x)
    print('  launch: %s' % (launch.group(1) if launch else '-'))
    print('  xml:    %s' % p)
    shown += 1
print('=' * 70)
print('%d toast(s) in the last %g hour(s)' % (shown, hours))
