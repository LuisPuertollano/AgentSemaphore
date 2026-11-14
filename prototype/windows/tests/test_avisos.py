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


def payload(*texts):
    body = ''.join('<text>%s</text>' % t for t in texts)
    return ('<toast launch="x"><visual><binding template="ToastGeneric">%s</binding></visual></toast>' % body).encode()


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

    def add(self, *texts, handler=1, minutes_ago=5, kind='toast'):
        self.db.execute('insert into Notification values (?, ?, ?, ?, ?)',
                        (self.next_id, handler, kind, filetime(NOW - datetime.timedelta(minutes=minutes_ago)), payload(*texts)))
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

    def test_missing_database_is_an_error(self):
        os.remove(os.path.join(self.src, 'wpndatabase.db'))
        rc, out = self.run_avisos()
        self.assertEqual(rc, 1)
        self.assertFalse(out['ok'])


if __name__ == '__main__':
    unittest.main()
