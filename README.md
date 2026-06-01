## First-time setup

```bash
task hooks:install           # Install lefthook git hooks (idempotent)
go work sync                 # Materialise workspace module set
task test                    # Run unit + adapter tests
```

## Local infrastructure

`task infra:up` brings up the entire local data plane and observability backend
in one command and blocks until every container reports healthy. It runs
Postgres (with a per-service database created for each service), Redpanda
(Kafka API + schema registry), Redis, OpenSearch, MinIO, and the LGTM stack
(Loki, Grafana, Tempo, Mimir) fanned in through an OpenTelemetry Collector, with
Grafana Alloy shipping container stdout to Loki. `task infra:down` stops it while
keeping data volumes; `task infra:down:hard` also wipes the volumes. `task
infra:ps` and `task infra:logs` inspect the running stack.

Host-exposed ports (everything else is reachable only on the `online-shop`
docker network):

| Port | Service |
|---|---|
| 5432 | Postgres |
| 9092 | Redpanda (Kafka) |
| 18081 | Redpanda schema registry |
| 9644 | Redpanda admin API |
| 6379 | Redis |
| 9200 | OpenSearch |
| 9000 | MinIO (S3 API) |
| 4317 / 4318 | OTel Collector (OTLP gRPC / HTTP) |
| 3100 | Loki |

Web UIs:

- **Grafana** — http://localhost:3000 (admin / admin)
- **Redpanda Console** — http://localhost:8085
- **MinIO Console** — http://localhost:9001 (minio / minio12345)
- **Alloy** — http://localhost:12345

The full stack is ~3 GB resident. On macOS, give Docker Desktop at least 8 GB of
RAM (Settings → Resources) or OpenSearch and the LGTM containers will be OOM-killed.

## Verify the bootstrap

```bash
task --list                  # List available task entrypoints
buf lint                     # Lint .proto definitions
golangci-lint config verify  # Validate linter configuration
task smoketest:run           # Emit one JSON log line + one OTel span
```

## Toolchain prerequisites

- Go 1.26+
- [Task](https://taskfile.dev/) (`brew install go-task`)
- [Buf](https://buf.build/docs/installation) (`brew install bufbuild/buf/buf`)
- [golangci-lint](https://golangci-lint.run/) (`brew install golangci-lint`)
- [lefthook](https://lefthook.dev/) (`brew install lefthook`)
- [goimports](https://pkg.go.dev/golang.org/x/tools/cmd/goimports) (`go install golang.org/x/tools/cmd/goimports@latest`)
- Docker + Docker Compose

Additional tooling — `sqlc`, `golang-migrate`, `air`, and `rover` (Apollo) — is
introduced as the corresponding subsystems land.

## Git hooks (lefthook)

`task hooks:install` registers pre-commit, commit-msg, and pre-push hooks that
enforce formatting, linting, and Conventional Commits. Commit messages must
match:

```
<type>(<scope>)?(!)?: <subject>
types: feat fix chore refactor docs test build ci perf
```

### Bypass policy

The hooks are a **courtesy, not a gate** — CI is the source of truth. During
WIP rebases or temporary commits you can skip them:

- `LEFTHOOK=0 git commit ...` — disables every lefthook hook for one command
- `git commit --no-verify` — git's stdlib bypass
- `lefthook-local.yml` (gitignored) — per-developer overrides

Pushing code that fails the hooks will still fail in CI; don't rely on the
bypass for anything you intend to merge.
