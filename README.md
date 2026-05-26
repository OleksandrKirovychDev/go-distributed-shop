# Online Shop

An educational Go-based distributed e-commerce backend, built as a learning
exercise in production-grade microservice patterns: hexagonal architecture,
gRPC + Kafka, Apollo Federation v2 GraphQL, the transactional outbox pattern,
and the LGTM observability stack.

## First-time setup

```bash
task hooks:install           # Install lefthook git hooks (idempotent)
go work sync                 # Materialise workspace module set
task test                    # Run unit + adapter tests
```

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