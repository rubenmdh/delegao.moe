import importlib.util
from contextlib import closing
from pathlib import Path
import sqlite3
import subprocess
import sys
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("repair_encoding", Path(__file__).with_name("repair-encoding.py"))
repair = importlib.util.module_from_spec(spec)
spec.loader.exec_module(repair)


class RepairEncodingTests(unittest.TestCase):
    def test_correct_unicode_and_markdown_are_preserved(self):
        text = "¡Hola, Rubén! ✅ ✔️ ⏳ — 中文 日本語 😀 François © £ **Markdown** &#x20; &amp; https://example.com/ñ"
        self.assertEqual(repair.repair_text(text), text)

    def test_latin1_cp1252_and_mixed_existing_content(self):
        original = "Rubén ✔️ ⏳ I’d — 😀"
        for codec in ("latin1", "cp1252"):
            broken = original.encode("utf-8").decode(codec, errors="surrogateescape")
            # MySQL/legacy systems may preserve undefined CP1252 bytes as controls.
            broken = "".join(chr(ord(c) - 0xDC00) if 0xDC80 <= ord(c) <= 0xDCFF else c for c in broken)
            self.assertEqual(repair.repair_text(broken), original)
            mixed = "Correcto: España 😀. Roto: " + broken
            fixed = "Correcto: España 😀. Roto: " + original
            self.assertEqual(repair.repair_text(mixed), fixed)
            self.assertEqual(repair.repair_text(fixed), fixed)

    def test_double_encoding(self):
        original = "Rubén ⏳"
        broken = original.encode().decode("latin1").encode().decode("latin1")
        self.assertEqual(repair.repair_text(broken), original)

    def test_reported_post_examples(self):
        self.assertEqual(repair.repair_text("âœ”ï¸\x8f â\x8f³ Iâ€™d â€” RubÃ©n &#x20;"),
                         "✔️ ⏳ I’d — Rubén &#x20;")

    def test_preview_excerpt_when_repaired_text_is_a_prefix(self):
        old = "Ãƒ"
        new = repair.repair_text(old)
        self.assertEqual(new, "Ã")
        self.assertEqual(repair.excerpt_pair(old, new), (old, new))

    def test_invalid_sequences_remain_unchanged(self):
        for text in ("Ã", "â", "ÃZ", "À©", "í\xa0\x80", "ô\x90\x80\x80", "�"):
            self.assertEqual(repair.repair_text(text), text)

    def fixture(self, directory):
        path = Path(directory) / "site.sqlite"
        with closing(sqlite3.connect(path)) as connection, connection:
            connection.execute("PRAGMA journal_mode=WAL")
            migration = Path(__file__).parent.parent / "migrations" / "001_sqlite.sql"
            connection.executescript(migration.read_text())
            connection.execute("INSERT INTO users(id,username,password) VALUES (7,'admin','unchanged-hash')")
            connection.execute("INSERT INTO news(id,title,body,status,tags,posted_by,last_update) VALUES (42,?,?,1,'literal &amp;',?,'2020-01-02 03:04:05')",
                               ("Rubén".encode().decode("latin1"), "Correcto: España 😀. Roto: " + "⏳ —".encode().decode("latin1"), "Rubén"))
            connection.execute("INSERT INTO guestbook(id,name,email,comment,country) VALUES (13,'Visitor','mail@example.com',?,'España')",
                               ("✔️".encode().decode("latin1"),))
        return path

    def test_preview_backup_wal_apply_and_repeat(self):
        with tempfile.TemporaryDirectory() as directory:
            path = self.fixture(directory)
            # Keep a WAL connection open and add a committed comment absent from
            # the original fixture: the online backup must include this too.
            with closing(sqlite3.connect(path)) as live, live:
                live.execute("INSERT INTO guestbook(id,name,email,comment,country) VALUES (14,'New visitor','new@example.com','Already correct 😀','España')")
                live.commit()
                before = list(live.iterdump())
                changes, backup = repair.run(path)
                self.assertEqual(len(changes), 3)
                self.assertIsNone(backup)
                self.assertEqual(list(live.iterdump()), before)
                changes, backup = repair.run(path, apply=True)
                with closing(sqlite3.connect(backup)) as saved:
                    self.assertEqual(list(saved.iterdump()), before)
                self.assertEqual(live.execute("SELECT title,body,tags,posted_by,last_update FROM news").fetchone(),
                                 ("Rubén", "Correcto: España 😀. Roto: ⏳ —", "literal &amp;", "Rubén", "2020-01-02 03:04:05"))
                self.assertEqual(live.execute("SELECT username,password FROM users").fetchone(), ("admin", "unchanged-hash"))
                self.assertEqual(live.execute("SELECT COUNT(*) FROM guestbook").fetchone(), (2,))
                self.assertEqual(live.execute("SELECT comment FROM guestbook WHERE id=14").fetchone(), ("Already correct 😀",))
                self.assertEqual(repair.run(path, apply=True), ([], None))

    def test_write_failure_rolls_back_all_fields(self):
        with tempfile.TemporaryDirectory() as directory:
            path = self.fixture(directory)
            with closing(sqlite3.connect(path)) as connection, connection:
                connection.execute("CREATE TRIGGER reject_guest BEFORE UPDATE ON guestbook BEGIN SELECT RAISE(ABORT,'simulated failure'); END")
                before = list(connection.iterdump())
            with self.assertRaises(sqlite3.IntegrityError):
                repair.run(path, apply=True)
            with closing(sqlite3.connect(path)) as connection:
                self.assertEqual(list(connection.iterdump()), before)
            self.assertEqual(len(list((path.parent / "encoding-backups").glob("*.sqlite"))), 1)

    def test_cli_preview_and_apply(self):
        with tempfile.TemporaryDirectory() as directory:
            path = self.fixture(directory)
            command = [sys.executable, str(Path(__file__).with_name("repair-encoding.py")), str(path)]
            preview = subprocess.run(command, capture_output=True, check=True).stdout.decode("utf-8")
            self.assertIn("Preview: 3 fields in 2 rows.", preview)
            self.assertIn("No data changed.", preview)
            applied = subprocess.run(command + ["--apply"], capture_output=True, check=True).stdout.decode("utf-8")
            self.assertIn("Repaired: 3 fields in 2 rows.", applied)
            self.assertIn("Backup:", applied)
            self.assertIn("Rubén", applied)


if __name__ == "__main__":
    unittest.main()
