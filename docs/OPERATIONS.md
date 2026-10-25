# Operations

Target: OCI Ampere A1 ARM64, 2 OCPU/12 GiB. Keep PostgreSQL private, bind health endpoints to loopback, limit workers to three, and retain 30% memory headroom. Back up with `pg_dump` daily to encrypted off-instance storage; restore into a separate database and run the smoke test weekly. Never blindly down-migrate production.

Metrics: `/healthz` and `/metrics` are intentionally bound to `HTTP_LISTEN` (default loopback). Alert on failed jobs, queue lag, disk, and memory.
