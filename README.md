# Online Shop

A from-scratch e-commerce backend built as a Go monorepo of federated
microservices — a deliberate practice ground for production-grade distributed
systems patterns.

## Motivation

This repository is a build-it-for-real exercise: an online-shop backend assembled
the way a production system would be, not a tutorial toy. The goal is to
internalise the patterns that hold up under real load and real teams:

- **Hexagonal services** — domain logic isolated from transport and storage, with
  the layering enforced at lint time (`depguard`) rather than left to convention.
- **Typed contracts everywhere** — gRPC + Protobuf (Buf) between services, a
  federated GraphQL edge (Apollo Federation v2) for clients.
- **A single secure entry point** — the Apollo Router validates the edge JWT
  against the issuer's JWKS and forwards the authenticated principal to subgraphs,
  rejecting bad tokens before any service is touched.
- **Event-driven by default** — Redpanda (Kafka API + schema registry) is part of
  the local data plane from day one.
- **Observability as a first-class concern** — structured logs, traces, and metrics
  flow through an OpenTelemetry Collector into the LGTM stack (Loki, Grafana,
  Tempo, Mimir).

Every subsystem is added deliberately, with explicit acceptance criteria, so each
piece is a unit of practice rather than a corner cut.

## Quick Start

Install the [toolchain](#toolchain-prerequisites), then:

```bash
task hooks:install   # Install lefthook git hooks (idempotent)
go work sync         # Materialise the workspace module set
task test            # Run unit + adapter tests

task up              # Build + start infra and services, block until healthy
```

`task up` brings up the data plane, observability stack, the identity service, and
the Apollo Router edge in one command. The GraphQL endpoint is then at
**http://localhost:4000/graphql** and the interactive Apollo Sandbox at
**http://localhost:4000**. To run only the infrastructure (no application
services), use `task infra:up` — see [Local infrastructure](#local-infrastructure).

> The router's JWT validation is a licensed Apollo GraphOS feature and needs
> `APOLLO_KEY` / `APOLLO_GRAPH_REF` (the free plan suffices). Copy `.env.example`
> to `.env` and fill them in before `task up`, or the router refuses to start.

## Usage

### The GraphQL edge

All client traffic goes through the router at `http://localhost:4000/graphql`.
Registration and login are public; `me` requires a valid access token.

```graphql
mutation Register {
  registerUser(input: { email: "a@example.com", password: "hunter2-strong" }) {
    id
    email
    roles
  }
}

mutation Login {
  loginUser(input: { email: "a@example.com", password: "hunter2-strong" }) {
    accessToken
    refreshToken
    accessTokenExpiresAt
  }
}
```

Send the `accessToken` as an `Authorization: Bearer <token>` header; the router
validates it and forwards the principal to the subgraph, which resolves `me`:

```graphql
query Me {
  me {
    id
    email
    roles
  }
}
```

### Common tasks

`task --list` shows every entrypoint. The ones you'll reach for most:

| Task | What it does |
|---|---|
| `task up` / `task down` | Start / stop the full stack (infra + services) |
| `task infra:up` / `task infra:down` | Start / stop infrastructure only |
| `task test` / `task test:integration` | Unit + adapter tests / testcontainer integration tests |
| `task lint` / `task fmt` | Lint / format every workspace module |
| `task proto:gen` / `task proto:lint` | Regenerate / lint gRPC code from `.proto` |
| `task supergraph:compose` | Recompose the federated supergraph from subgraph schemas |
| `task identity:migrate` | Apply identity database migrations |
| `task identity:run` | Run the identity service on the host (hot-reload via air) |

Service-scoped tasks live under their own namespace; run `task identity:` to list
them (`identity:sqlc`, `identity:graphql:gen`, `identity:keygen`, …).

> Running a service on the host (`task identity:run`) points it at the compose
> Postgres on `localhost:5432`. A local Postgres install (Postgres.app/Homebrew)
> shadows that port and fails with `role "postgres" does not exist` — stop it, or
> exercise the service inside the stack with `task up`.

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

## Contributing

CI is the source of truth; the local hooks are a fast feedback loop that mirrors
it. Before opening a PR, make sure `task fmt`, `task lint`, and `task test` pass.

### Conventions

- **Commits follow Conventional Commits**, enforced by the `commit-msg` hook:
  ```
  <type>(<scope>)?(!)?: <subject>
  types: feat fix chore refactor docs test build ci perf
  ```
- **Hexagonal layering is a hard rule** — domain code never imports transport or
  storage; it is enforced at lint time via `depguard`, not left to discipline.
- **Protobuf and GraphQL are contracts** — regenerate (`task proto:gen`,
  `task identity:graphql:gen`, `task supergraph:compose`) and commit the output.
  `task proto:breaking` and `task supergraph:check` guard against drift.

### Git hooks (lefthook)

`task hooks:install` registers pre-commit, commit-msg, and pre-push hooks that
enforce formatting, linting, and Conventional Commits.

#### Bypass policy

The hooks are a **courtesy, not a gate** — CI is the source of truth. During
WIP rebases or temporary commits you can skip them:

- `LEFTHOOK=0 git commit ...` — disables every lefthook hook for one command
- `git commit --no-verify` — git's stdlib bypass
- `lefthook-local.yml` (gitignored) — per-developer overrides

Pushing code that fails the hooks will still fail in CI; don't rely on the
bypass for anything you intend to merge.
