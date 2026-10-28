#!/usr/bin/env python3
import os
import sqlite3
import sys
import time
from datetime import datetime
from pathlib import Path
from zoneinfo import ZoneInfo

ROOT = Path("/home/ubuntu/PelagornisBot")
SOURCE = ROOT / "data/pelagornisbot.db"
BACKUPS = ROOT / "backups"
RETENTION_SECONDS = 14 * 24 * 60 * 60

def main():
    if not SOURCE.is_file():
        raise RuntimeError(f"Database not found: {SOURCE}")

    BACKUPS.mkdir(parents=True, exist_ok=True)
    os.chmod(BACKUPS, 0o750)

    timestamp = datetime.now(ZoneInfo("Asia/Seoul")).strftime("%Y%m%d-%H%M%S")
    target = BACKUPS / f"pelagornisbot-{timestamp}.db"
    temporary = BACKUPS / f".pelagornisbot-{timestamp}.tmp"

    try:
        # SQLite backup API safely handles a database using WAL mode.
        with sqlite3.connect(str(SOURCE), timeout=30) as src:
            with sqlite3.connect(str(temporary), timeout=30) as dst:
                src.backup(dst)

        with sqlite3.connect(str(temporary)) as check:
            result = check.execute("PRAGMA integrity_check").fetchone()[0]

        if result != "ok":
            raise RuntimeError(f"SQLite integrity check failed: {result}")

        os.chmod(temporary, 0o600)
        os.replace(temporary, target)
        print(f"Backup OK: {target} ({target.stat().st_size:,} bytes)")
        print("Integrity: ok")

        # Prune old backups only after the new backup passes verification.
        cutoff = time.time() - RETENTION_SECONDS
        for old in BACKUPS.glob("pelagornisbot-*.db"):
            if old != target and old.stat().st_mtime < cutoff:
                old.unlink()
                print(f"Removed expired backup: {old.name}")

    finally:
        temporary.unlink(missing_ok=True)

if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(f"BACKUP FAILED: {exc}", file=sys.stderr)
        sys.exit(1)
