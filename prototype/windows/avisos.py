# Lee (en una copia) la base de avisos de Windows y devuelve, en JSON, los avisos
# recientes de la app Claude que pueden pedir tu atencion (chat, Cowork, tareas).
# Lo usa semaforo.ps1. Solo lectura: nunca toca la base original.
#
# Salida (stdout, una linea JSON):
#   {"ok": true, "avisos": [{"id", "epoch", "nombre", "textos", "tipo", "origen", "silencioso"}]}
#     tipo = "pide"        -> el texto coincide con una frase conocida de "te necesita"
#            "desconocido" -> aviso de Claude con un texto que no esta en avisos-textos.json;
#                             cuenta como rojo (mejor un rojo de mas que un verde falso)
#   origen     = "nube"  -> aviso de una sesion de Claude Code en la nube (<header id="session-session_...">)
#                "local" -> sesion de la pestana Code, local o por SSH (<header id="session-local_...">)
#                ""      -> otro aviso (chat, Cowork, tareas)
#   silencioso = el aviso trae <audio silent="true"/>. Visto solo en un resumen de turno que no
#                pedia nada (2026-11-14); se registra para comprobarlo, todavia no cambia el color.
#   Si algo falla: {"ok": false, "error": "..."} y codigo de salida 1.
#
# Variables para pruebas: AGENTSEMAPHORE_WPN_DIR (carpeta de la base),
# AGENTSEMAPHORE_TMP (carpeta de la copia), AGENTSEMAPHORE_NOW_EPOCH (hora "actual").
import datetime, json, os, re, shutil, sqlite3, sys, tempfile

AUMID = 'Claude_pzs8sxrjxfjjc!Claude'
HORAS = 12
AQUI = os.path.dirname(os.path.abspath(__file__))


def cargar_textos(ruta=os.path.join(AQUI, 'avisos-textos.json')):
    with open(ruta, encoding='utf-8') as f:
        d = json.load(f)
    pide, ignorar = [], []
    for idioma, v in d.get('idiomas', {}).items():
        pide += v.get('pide', [])
        ignorar += v.get('ignorar', [])
    if not pide:
        raise ValueError('avisos-textos.json sin frases "pide"')
    return re.compile('|'.join(pide), re.I), (re.compile('|'.join(ignorar), re.I) if ignorar else None)


def copiar_base():
    src = os.environ.get('AGENTSEMAPHORE_WPN_DIR') or os.path.join(
        os.environ['LOCALAPPDATA'], 'Microsoft', 'Windows', 'Notifications')
    tmp = os.environ.get('AGENTSEMAPHORE_TMP') or os.path.join(tempfile.gettempdir(), 'semaforo_wpn')
    os.makedirs(tmp, exist_ok=True)
    for n in ('wpndatabase.db-shm', 'wpndatabase.db-wal', 'wpndatabase.db'):
        p = os.path.join(tmp, n)
        if os.path.exists(p):
            os.remove(p)
    shutil.copyfile(os.path.join(src, 'wpndatabase.db'), os.path.join(tmp, 'wpndatabase.db'))
    wal = os.path.join(src, 'wpndatabase.db-wal')
    if os.path.exists(wal):
        shutil.copyfile(wal, os.path.join(tmp, 'wpndatabase.db-wal'))
    return os.path.join(tmp, 'wpndatabase.db')


def texto_payload(payload):
    return payload.decode('utf-8', 'replace') if isinstance(payload, bytes) else str(payload)


def textos_de(p):
    return [re.sub(r'<[^>]+>', '', x).strip() for x in re.findall(r'<text[^>]*>(.*?)</text>', p, re.S)]


def origen_de(p):
    m = re.search(r'<header\b[^>]*\bid="session-([a-z]+)_', p)
    if not m:
        return ''
    return {'session': 'nube', 'local': 'local'}.get(m.group(1), m.group(1))


def silencioso(p):
    return re.search(r'<audio\b[^>]*\bsilent="true"', p) is not None


def main():
    pide, ignorar = cargar_textos()
    c = sqlite3.connect(copiar_base())
    try:
        rows = c.execute(
            "select n.Id, n.ArrivalTime, n.Payload from Notification n "
            "join NotificationHandler h on h.RecordId = n.HandlerId "
            "where h.PrimaryId = ? and n.Type = 'toast' "
            "order by n.ArrivalTime desc limit 40", (AUMID,)).fetchall()
    finally:
        c.close()
    ahora = os.environ.get('AGENTSEMAPHORE_NOW_EPOCH')
    ahora = (datetime.datetime(1970, 1, 1) + datetime.timedelta(seconds=int(ahora))) if ahora else \
        datetime.datetime.now(datetime.timezone.utc).replace(tzinfo=None)
    limite = ahora - datetime.timedelta(hours=HORAS)
    out = []
    for nid, arr, payload in rows:
        t = datetime.datetime(1601, 1, 1) + datetime.timedelta(microseconds=arr / 10)
        if t < limite:
            continue
        p = texto_payload(payload)
        textos = textos_de(p)
        if any(pide.search(x) for x in textos):
            tipo = 'pide'
        elif ignorar and any(ignorar.search(x) for x in textos):
            continue
        else:
            tipo = 'desconocido'
        # El nombre es el texto que NO es una frase fija del aviso
        fijo = lambda x: pide.search(x) or (ignorar and ignorar.search(x))
        nombre = next((x for x in textos if x and not fijo(x)), textos[0] if textos else 'Claude')
        out.append({'id': nid, 'epoch': int((t - datetime.datetime(1970, 1, 1)).total_seconds()),
                    'nombre': nombre, 'textos': textos, 'tipo': tipo,
                    'origen': origen_de(p), 'silencioso': silencioso(p)})
    print(json.dumps({'ok': True, 'avisos': out}, ensure_ascii=True))


if __name__ == '__main__':
    try:
        main()
    except Exception as e:
        print(json.dumps({'ok': False, 'error': '%s: %s' % (type(e).__name__, e)}, ensure_ascii=True))
        sys.exit(1)
