"""Tests for scoop.py: python3 -m unittest discover -s tools/release"""
import pathlib
import tempfile
import unittest

import scoop

NAMES = ["agentsemaphore-windows-amd64.exe", "agentsemaphore-windows-arm64.exe",
         "agentsemaphore-cli-windows-amd64.exe", "agentsemaphore-cli-windows-arm64.exe",
         "agentsemaphore-linux-amd64"]


class ScoopTest(unittest.TestCase):
    def dist(self, names):
        d = pathlib.Path(tempfile.mkdtemp())
        lines = [f"{i:064x}  {n}" for i, n in enumerate(names, start=1)]
        (d / "SHA256SUMS.txt").write_text("\n".join(lines) + "\n")
        return d

    def test_manifest(self):
        m = scoop.manifest("v0.1.0-alpha.4", scoop.read_sums(self.dist(NAMES)))
        self.assertEqual(m["version"], "0.1.0-alpha.4")
        a = m["architecture"]["arm64"]
        self.assertEqual(a["url"][0], "https://github.com/LuisPuertollano/AgentSemaphore/releases/download/"
                                      "v0.1.0-alpha.4/agentsemaphore-windows-arm64.exe#/agentsemaphore.exe")
        self.assertTrue(a["url"][1].endswith("#/agentsemaphore-cli.exe"))
        self.assertEqual(a["hash"], [f"{2:064x}", f"{4:064x}"])
        self.assertEqual(m["bin"], [["agentsemaphore-cli.exe", "agentsemaphore"]])

    def test_missing_console_build_fails(self):
        with self.assertRaises(SystemExit):
            scoop.manifest("v0.1.0-alpha.2", scoop.read_sums(self.dist(NAMES[:2])))

    def test_bad_tag_fails(self):
        with self.assertRaises(SystemExit):
            scoop.manifest("latest", scoop.read_sums(self.dist(NAMES)))


if __name__ == "__main__":
    unittest.main()
