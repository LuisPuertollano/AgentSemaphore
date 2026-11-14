"""Tests for avisos.py against a synthetic copy of the Windows notification DB.

Only the columns avisos.py reads are created. Runs on any OS (CI: ubuntu).
"""
import datetime
import json
import os
import sqlite3
import subprocess
import sys
import tempfile
import unittest

AVISOS = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), 'avisos.py')
NOW = datetime.datetime(2026, 11, 14, 12, 0, 0)
AUMID = 'Claude_pzs8sxrjxfjjc!Claude'


def filetime(dt):
    return int((dt - datetime.datetime(1601, 1, 1)).total_seconds() * 10_000_000)


def payload(*texts, header='', silent=False):
    body = ''.join('<text>%s</text>' % t for t in texts)
    audio = '<audio silent="true"/>' if silent else ''
    return ('<toast>%s<visual><binding template="ToastGeneric">%s</binding></visual>%s</toast>' % (header, body, audio)).encode()


class AvisosTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.src = os.path.join(self.tmp.name, 'src')
        os.makedirs(self.src)
        self.db = sqlite3.connect(os.path.join(self.src, 'wpndatabase.db'))
        self.db.execute('create table NotificationHandler (RecordId integer primary key, PrimaryId text)')
        self.db.execute('create table Notification (Id integer primary key, HandlerId integer, Type text, ArrivalTime integer, Payload blob)')
        self.db.execute('insert into NotificationHandler values (1, ?)', (AUMID,))
        self.db.execute("insert into NotificationHandler values (2, 'Other.App!App')")
        self.next_id = 1

    def tearDown(self):
        self.db.close()
        self.tmp.cleanup()

    def add(self, *texts, handler=1, minutes_ago=5, kind='toast', header='', silent=False):
        self.db.execute('insert into Notification values (?, ?, ?, ?, ?)',
                        (self.next_id, handler, kind, filetime(NOW - datetime.timedelta(minutes=minutes_ago)),
                         payload(*texts, header=header, silent=silent)))
        self.db.commit()
        self.next_id += 1

    def run_avisos(self):
        env = dict(os.environ,
                   AGENTSEMAPHORE_WPN_DIR=self.src,
                   AGENTSEMAPHORE_TMP=os.path.join(self.tmp.name, 'copy'),
                   AGENTSEMAPHORE_NOW_EPOCH=str(int((NOW - datetime.datetime(1970, 1, 1)).total_seconds())))
        p = subprocess.run([sys.executable, '-I', AVISOS], env=env, capture_output=True, text=True)
        return p.returncode, json.loads(p.stdout)

    def test_known_phrases_in_three_languages(self):
        self.add('Mi chat', 'Claude necesita tu respuesta para continuar')
        self.add('My chat', 'Claude needs your response to continue')
        self.add('Mein Chat', 'Claude wartet auf deine Antwort')
        rc, out = self.run_avisos()
        self.assertEqual(rc, 0)
        got = {(a['nombre'], a['tipo']) for a in out['avisos']}
        self.assertEqual(got, {('Mi chat', 'pide'), ('My chat', 'pide'), ('Mein Chat', 'pide')})

    def test_ignored_and_unknown(self):
        self.add('Tarea programada completada', 'Resumen diario')
        self.add('Something new from Claude')
        rc, out = self.run_avisos()
        self.assertEqual(rc, 0)
        self.assertEqual([(a['nombre'], a['tipo']) for a in out['avisos']],
                         [('Something new from Claude', 'desconocido')])

    def test_filters_other_apps_old_and_non_toast(self):
        self.add('X', 'Claude necesita tu respuesta', handler=2)
        self.add('Y', 'Claude necesita tu respuesta', minutes_ago=13 * 60)
        self.add('Z', 'Claude necesita tu respuesta', kind='tile')
        rc, out = self.run_avisos()
        self.assertEqual((rc, out['avisos']), (0, []))

    def test_schema_change_is_an_error_not_an_empty_list(self):
        self.db.execute('alter table Notification rename to Notification2')
        self.db.commit()
        rc, out = self.run_avisos()
        self.assertEqual(rc, 1)
        self.assertFalse(out['ok'])
        self.assertIn('Notification', out['error'])

    def test_exact_package_texts_in_every_language(self):
        # The exact strings shipped in the Claude Desktop package must classify as listed.
        with open(os.path.join(os.path.dirname(AVISOS), 'avisos-textos.json'), encoding='utf-8') as f:
            langs = json.load(f)['idiomas']
        expected = {}
        for lang, v in langs.items():
            for kind in ('pide', 'ignorar'):
                for text in v['ejemplos'][kind]:
                    self.add('Chat %s %d' % (lang, self.next_id), text)
                    expected['Chat %s %d' % (lang, self.next_id - 1)] = kind
        rc, out = self.run_avisos()
        self.assertEqual(rc, 0)
        got = {a['nombre']: a['tipo'] for a in out['avisos']}
        for name, kind in expected.items():
            if kind == 'pide':
                self.assertEqual(got.get(name), 'pide', name)
            else:
                self.assertNotIn(name, got, name)

    def test_code_session_toasts_report_origin_and_silence(self):
        # Shapes seen on 2026-11-14 (titles and ids made up): turn-end summaries of Code sessions.
        self.add('Cloud session', 'please confirm step 4',
                 header='<header id="session-session_01ABC" title="Cloud session" arguments=""/>')
        self.add('Local session', 'deleted three containers; backups kept',
                 header='<header id="session-local_5ad85bbc-0000" title="Local session" arguments=""/>', silent=True)
        self.add('Some chat', 'Claude necesita tu respuesta para continuar')
        rc, out = self.run_avisos()
        self.assertEqual(rc, 0)
        got = {a['nombre']: (a['tipo'], a['origen'], a['silencioso']) for a in out['avisos']}
        self.assertEqual(got['Cloud session'], ('desconocido', 'nube', False))
        # Still red (desconocido) for now: silence is only recorded, not trusted yet.
        self.assertEqual(got['Local session'], ('desconocido', 'local', True))
        self.assertEqual(got['Some chat'], ('pide', '', False))

    def test_missing_database_is_an_error(self):
        os.remove(os.path.join(self.src, 'wpndatabase.db'))
        rc, out = self.run_avisos()
        self.assertEqual(rc, 1)
        self.assertFalse(out['ok'])


if __name__ == '__main__':
    unittest.main()
