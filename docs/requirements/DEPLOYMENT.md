# Deployment Requirements

## Docker

- DEPLOY-1: The server MUST be packaged as a Docker image using a multi-stage build.
- DEPLOY-2: The Docker image MUST use a minimal base image (distroless or Alpine).
- DEPLOY-3: The project MUST provide a `docker-compose.yml` for local development that includes the server, PostgreSQL, etcd, and NATS.
- DEPLOY-4: Configuration MUST be injectable via environment variables and/or mounted YAML configuration files.
- DEPLOY-5: Game content (YAML data files, Lua scripts) MUST be mountable as Docker volumes.

## Clustering

- DEPLOY-6: The server MUST support horizontal scaling by running multiple Pitaya server instances behind etcd service discovery.
- DEPLOY-7: Frontend server instances MUST be stateless with respect to game logic; all state MUST reside in backend servers or PostgreSQL.
- DEPLOY-8: Zone assignment to backend servers MUST be configurable and support rebalancing.
- DEPLOY-9: The cluster MUST tolerate the loss of a single backend server instance with automatic zone reassignment.

## Observability

- DEPLOY-10: The server MUST expose Prometheus-compatible metrics for: connected players, active zones, tick rate, command throughput, database query latency, and RPC latency.
- DEPLOY-11: The server MUST emit structured logs (JSON format) to stdout.
- DEPLOY-12: The server MUST support OpenTelemetry tracing for request flow across frontend and backend servers.
- DEPLOY-13: The server MUST expose a health check endpoint for container orchestration liveness and readiness probes.

## Performance

- DEPLOY-14: The server MUST maintain its target tick rate under load with 500 concurrent players and 1000 active NPCs.
- DEPLOY-15: Command-to-response latency MUST NOT exceed 100ms at the 99th percentile under normal operating conditions.
- DEPLOY-16: Database write operations MUST be batched where possible to minimize round-trips.

## Security

- DEPLOY-17: The server MUST NOT expose database ports or internal RPC ports to external networks.
- DEPLOY-18: All inter-server communication MUST support TLS encryption in production deployments.
- DEPLOY-19: The Docker image MUST NOT run as root.
- DEPLOY-20: The server MUST enforce rate limiting on authentication attempts.
