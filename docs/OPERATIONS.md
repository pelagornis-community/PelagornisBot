# Operations

Target: OCI E2.1.Micro or larger. SQLite runs in WAL mode with one PelagornisBot instance and one worker; bind health endpoints to loopback. Back up the SQLite database with `sqlite3 data/pelagornisbot.db '.backup backup/pelagornisbot.db'` daily to encrypted off-instance storage, then restore a copy and run the smoke test weekly. Never blindly down-migrate production.

Metrics: `/healthz` and `/metrics` are intentionally bound to `HTTP_LISTEN` (default loopback). Alert on failed jobs, queue lag, disk, and memory.
