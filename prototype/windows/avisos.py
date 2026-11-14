# Lee (en una copia) la base de avisos de Windows y devuelve, en JSON, los avisos
# recientes de la app Claude que piden tu respuesta o aprobacion (chat, Cowork, tareas).
# Lo usa semaforo.ps1. Solo lectura: nunca toca la base original.
import json, os, re, shutil, sqlite3, sys, tempfile, datetime

SRC = os.path.join(os.environ['LOCALAPPDATA'], 'Microsoft', 'Windows', 'Notifications')
TMP = os.path.join(tempfile.gettempdir(), 'semaforo_wpn')
PIDE = re.compile(r'necesita tu (respuesta|aprobaci)|esperando tu respuesta|needs your (response|input|approval)|waiting for your|wartet auf|ben.tigt deine', re.I)

def main():
    os.makedirs(TMP, exist_ok=True)
    for n in ('wpndatabase.db-shm', 'wpndatabase.db-wal', 'wpndatabase.db'):
        p = os.path.join(TMP, n)
        if os.path.exists(p): os.remove(p)
    shutil.copyfile(os.path.join(SRC, 'wpndatabase.db'), os.path.join(TMP, 'wpndatabase.db'))
    wal = os.path.join(SRC, 'wpndatabase.db-wal')
    if os.path.exists(wal): shutil.copyfile(wal, os.path.join(TMP, 'wpndatabase.db-wal'))
    c = sqlite3.connect(os.path.join(TMP, 'wpndatabase.db'))
    rows = c.execute(
        "select n.Id, n.ArrivalTime, n.Payload from Notification n "
        "join NotificationHandler h on h.RecordId = n.HandlerId "
        "where h.PrimaryId = 'Claude_pzs8sxrjxfjjc!Claude' and n.Type = 'toast' "
        "order by n.ArrivalTime desc limit 40").fetchall()
    limite = datetime.datetime.now(datetime.timezone.utc).replace(tzinfo=None) - datetime.timedelta(hours=12)
    out = []
    for nid, arr, payload in rows:
        t = datetime.datetime(1601, 1, 1) + datetime.timedelta(microseconds=arr / 10)
        if t < limite: continue
        p = payload.decode('utf-8', 'replace') if isinstance(payload, bytes) else str(payload)
        textos = [re.sub(r'<[^>]+>', '', x).strip() for x in re.findall(r'<text[^>]*>(.*?)</text>', p, re.S)]
        if not any(PIDE.search(x) for x in textos): continue
        # El nombre es el texto que NO es la frase fija del aviso
        nombre = next((x for x in textos if x and not PIDE.search(x)), textos[0] if textos else 'Claude')
        out.append({'id': nid, 'epoch': int((t - datetime.datetime(1970, 1, 1)).total_seconds()), 'nombre': nombre, 'textos': textos})
    print(json.dumps(out, ensure_ascii=True))

if __name__ == '__main__':
    try: main()
    except Exception as e:
        print(json.dumps({'error': str(e)}, ensure_ascii=True)); sys.exit(1)