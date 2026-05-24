# Online Shop

An educational Go-based distributed e-commerce backend, built as a learning
exercise in production-grade microservice patterns: hexagonal architecture,
gRPC + Kafka, Apollo Federation v2 GraphQL, the transactional outbox pattern,
and the LGTM observability stack.

## Verify the bootstrap

```bash
task --list                  # List available task entrypoints
buf lint                     # Lint .proto definitions
golangci-lint config verify  # Validate linter configuration
```

## Toolchain prerequisites

- Go 1.23+
- [Task](https://taskfile.dev/) (`brew install go-task`)
- [Buf](https://buf.build/docs/installation) (`brew install bufbuild/buf/buf`)
- [golangci-lint](https://golangci-lint.run/) (`brew install golangci-lint`)
- Docker + Docker Compose

Additional tooling — `sqlc`, `golang-migrate`, `air`, and `rover` (Apollo) — is
introduced as the corresponding subsystems land.
