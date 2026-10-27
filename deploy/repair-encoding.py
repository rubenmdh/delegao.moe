#!/usr/bin/env python3
"""Preview or repair reversible UTF-8 mojibake in an existing SQLite database.

Stop the Go service before --apply and restart it afterwards to regenerate HTML.
No HTML-entity decoding, normalization, credential updates or MySQL writes occur.
"""

import argparse
from contextlib import closing
import json
import os
from pathlib import Path
import sqlite3
import sys
import tempfile


FIELDS = {
    "news": ("title", "body", "tags", "posted_by"),
    "guestbook": ("name", "email", "comment", "country"),
}


def legacy_byte(char):
    """Reverse both Latin-1 controls and Windows-1252 display characters."""
    if ord(char) <= 255:
        return ord(char)
    try:
        return char.encode("cp1252")[0]
    except UnicodeEncodeError:
        return None


def repair_once(text):
    result = []
    i = 0
    while i < len(text):
        lead = legacy_byte(text[i])
        size = (2 if 0xC2 <= lead <= 0xDF else
                3 if 0xE0 <= lead <= 0xEF else
                4 if 0xF0 <= lead <= 0xF4 else 0) if lead is not None else 0
        if size and i + size <= len(text):
            octets = [legacy_byte(char) for char in text[i:i + size]]
            if all(value is not None and 0x80 <= value <= 0xBF for value in octets[1:]):
                try:
                    decoded = bytes(octets).decode("utf-8", errors="strict")
                except UnicodeDecodeError:
                    pass
                else:
                    result.append(decoded)
                    i += size
                    continue
        result.append(text[i])
        i += 1
    return "".join(result)


def repair_text(text):
    # Each successful replacement shortens the text. Bounded passes handle
    # double encoding; refuse pathological inputs rather than partially repair.
    for _ in range(8):
        repaired = repair_once(text)
        if repaired == text:
            return text
        text = repaired
    if repair_once(text) != text:
        raise ValueError("more than eight encoding layers; manual review required")
    return text


def changes_in(connection):
    changes = []
    for table, columns in FIELDS.items():
        for row in connection.execute(f"SELECT id,{','.join(columns)} FROM {table} ORDER BY id"):
            for column, old in zip(columns, row[1:]):
                if not isinstance(old, str):
                    raise ValueError(f"{table} id={row[0]} {column}: expected text")
                new = repair_text(old)
                if new != old:
                    changes.append((table, row[0], column, old, new))
    return changes


def connect_existing(path, readonly=False):
    return sqlite3.connect(path.as_uri() + ("?mode=ro" if readonly else "?mode=rw"),
                           uri=True, timeout=5, isolation_level=None)


def make_backup(path, directory):
    directory.mkdir(mode=0o700, parents=True, exist_ok=True)
    descriptor, filename = tempfile.mkstemp(prefix="before-encoding-", suffix=".sqlite", dir=directory)
    os.close(descriptor)
    # A separate read connection can back up the committed snapshot while the
    # repair connection holds BEGIN IMMEDIATE, excluding concurrent writers.
    with closing(connect_existing(path, readonly=True)) as source:
        with closing(sqlite3.connect(filename)) as target:
            source.backup(target)
            if target.execute("PRAGMA quick_check").fetchall() != [("ok",)]:
                raise RuntimeError("backup integrity check failed")
    return filename


def run(path, apply=False, backup_dir=None):
    path = Path(path).resolve(strict=True)
    if not path.is_file():
        raise ValueError("database must be an existing regular file")
    connection = connect_existing(path, readonly=not apply)
    backup = None
    try:
        connection.execute("BEGIN IMMEDIATE" if apply else "BEGIN")
        if connection.execute("PRAGMA quick_check").fetchall() != [("ok",)]:
            raise RuntimeError("source database integrity check failed")
        changes = changes_in(connection)
        if apply and changes:
            backup = make_backup(path, Path(backup_dir) if backup_dir else path.parent / "encoding-backups")
            for table, identity, column, old, new in changes:
                connection.execute(f"UPDATE {table} SET {column}=? WHERE id=?", (new, identity))
        connection.execute("COMMIT")
    finally:
        connection.close()
    return changes, backup


def excerpt_pair(old, new):
    different = next((i for i, (a, b) in enumerate(zip(old, new)) if a != b), min(len(old), len(new)))
    start = max(0, different - 24)
    return old[start:different + 80], new[start:different + 80]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("database", help="existing SQLite file, normally DATABASE_DSN")
    parser.add_argument("--apply", action="store_true", help="back up and apply; default only previews")
    parser.add_argument("--backup-dir", help="default: encoding-backups beside the database")
    args = parser.parse_args()
    changes, backup = run(args.database, args.apply, args.backup_dir)
    for table, identity, column, old, new in changes:
        before, after = excerpt_pair(old, new)
        print(json.dumps({"table": table, "id": identity, "field": column,
                          "before": before, "after": after}, ensure_ascii=False))
    rows = len({(table, identity) for table, identity, *_ in changes})
    print(f"{'Repaired' if args.apply else 'Preview'}: {len(changes)} fields in {rows} rows.")
    if backup:
        print(f"Backup: {backup}")
    if not args.apply:
        print("No data changed. Review the examples before running --apply with the service stopped.")


if __name__ == "__main__":
    sys.stdout.reconfigure(encoding="utf-8")
    try:
        main()
    except (sqlite3.Error, OSError, ValueError, RuntimeError) as error:
        print(f"Encoding repair failed: {error}", file=sys.stderr)
        sys.exit(1)
