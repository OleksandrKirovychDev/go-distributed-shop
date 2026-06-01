# Identity Service

Authentication and user identity for the online-shop platform: registration,
login (access + refresh tokens), token refresh/verification, and user lookup.

It is a **hexagonal (ports & adapters)** Go service with **two inbound
transports over the same application core**:

| Transport | Address (local) | Who talks to it | Purpose |
|---|---|---|---|
| **GraphQL** (federated subgraph) | `:8081/graphql` | The Apollo Router (edge) | Public-facing API: `registerUser`, `loginUser`, `me` |
| **gRPC** | `:50051` | Other backend services | Internal API: all 5 RPCs incl. `VerifyToken`, `RefreshToken`, `GetUser` |

Both adapters call the **exact same use cases** — they are thin translation
layers (proto/GraphQL ⇄ app DTOs). The app and domain never see proto, gqlgen,
pgx, or JWT types.

> The client never talks to this service directly. It talks to the **Apollo
> Router** (the GraphQL gateway), which validates the edge JWT and forwards the
> request to this service's `:8081/graphql` subgraph. See [Data flow](#data-flow).

---

## Entry files (start here)

### Identity service

| Concern | File |
|---|---|
| **Composition root / `main`** (config, wiring, both servers, graceful shutdown) | `services/identity/cmd/identity/main.go` |
| Inbound **GraphQL** resolvers | `services/identity/internal/adapters/inbound/graphql/schema.resolvers.go` |
| Inbound GraphQL auth middleware (reads forwarded principal) | `services/identity/internal/adapters/inbound/graphql/middleware.go` |
| Inbound **gRPC** server + per-RPC handlers | `services/identity/internal/adapters/inbound/grpc/server.go` (+ `*_handler.go`) |
| Application **use cases** (the actual business logic) | `services/identity/internal/app/{registeruser,loginuser,refreshtoken,verifytoken,getuser}/` |
| Inbound/outbound **port interfaces** + DTOs | `services/identity/internal/app/ports/{inbound,outbound}.go` |
| **Domain** (User, Email, Password, Role, RefreshToken, Token) | `services/identity/internal/domain/` |
| Outbound adapters (Postgres, Argon2, JWT, outbox) | `services/identity/internal/adapters/outbound/` |
| **GraphQL schema** (source of truth for the subgraph) | `services/identity/api/graphql/schema.graphql` |
| **gRPC/proto contract** | `proto/identity/v1/identity.proto` |
| SQL migrations / queries | `services/identity/migrations/`, `services/identity/queries/` |

### GraphQL gateway (Apollo Router — the client's entry point)

The gateway is **not** a Go service in this repo. It is the off-the-shelf
`ghcr.io/apollographql/router` image, configured entirely by files in
`deploy/router/`:

| Concern | File |
|---|---|
| Router config (listen `:4000/graphql`, JWT validation, telemetry) | `deploy/router/router.yaml` |
| **Principal forwarding** (JWT claims → `x-user-id` / `x-user-roles` headers) | `deploy/router/router.rhai` |
| Composed **supergraph** the router serves (committed, generated) | `deploy/router/supergraph.graphql` |
| Supergraph composition input (which subgraphs exist) | `deploy/router/supergraph-config.yaml` |
| Router container definition | `deploy/compose/services.yml` (`router` service) |

---

## Layout (hexagonal)

```
services/identity/
├── cmd/identity/main.go        ← composition root: wires everything, serves gRPC + HTTP
├── api/graphql/schema.graphql  ← subgraph schema (drives gqlgen codegen)
├── internal/
│   ├── domain/                 ← pure types & rules (Email, Password, User, Role, RefreshToken)
│   ├── app/
│   │   ├── ports/              ← inbound/outbound interfaces + DTOs (the "hexagon edges")
│   │   ├── registeruser/       ← use case: validate → hash → persist user + outbox event (1 tx)
│   │   ├── loginuser/          ← use case: verify creds → issue JWT + persist refresh token
│   │   ├── refreshtoken/       ← use case: rotate refresh token, issue new access token
│   │   ├── verifytoken/        ← use case: parse/validate a JWT (internal)
│   │   └── getuser/            ← use case: read a user by id
│   └── adapters/
│       ├── inbound/
│       │   ├── graphql/        ← gqlgen subgraph (resolvers, federation, error mapping)
│       │   └── grpc/           ← protobuf server (handlers, status mapping, metadata auth)
│       └── outbound/
│           ├── postgres/       ← sqlc-backed repos, tx manager, transactional outbox
│           ├── argon2/         ← password hashing + refresh-token generation
│           ├── jwt/            ← RS256 token issue/parse + JWKS endpoint (adapts pkg/auth)
│           ├── events/         ← proto event encoding for the outbox
│           └── system/         ← clock + ID generator
```

Dependency rule: `adapters → app(ports) → domain`. The arrows only point inward.
Adapters depend on port interfaces; nothing in `app`/`domain` imports an adapter.

---

## Data flow

### 1. GraphQL request through the gateway (the real client path)

```
                                            ┌──────────── identity service ────────────┐
client ──HTTP/JSON──> Apollo Router ──HTTP──> AuthMiddleware ─> gqlgen ─> Resolver ─> UseCase ─> outbound ─> Postgres
  (GraphQL)            :4000/graphql          (:8081/graphql)    resolver            (app)       adapters
                          │
                          ├─ validates edge JWT against identity's JWKS
                          │  (http://identity:8081/.well-known/jwks.json)
                          │  → present-but-invalid/expired ⇒ 401 before any subgraph call
                          │  → missing token ⇒ passes through anonymously
                          │
                          └─ router.rhai copies JWT claims onto subgraph headers:
                             sub   → x-user-id
                             roles → x-user-roles
```

- **`registerUser` / `loginUser`** are public — no token required; the router
  forwards them header-free and the resolver runs anonymously.
- **`me`** is protected. `AuthMiddleware`
  (`graphql/middleware.go`) lifts `x-user-id` / `x-user-roles` into the request
  context; the resolver (`schema.resolvers.go:47`) rejects with
  `UNAUTHENTICATED` if no caller is present, otherwise looks the user up by the
  forwarded id.

> **Why the subgraph trusts a header:** the router already cryptographically
> validated the JWT at the edge. The subgraph is only reachable on the internal
> docker network, so it consumes the verified principal rather than re-parsing
> the token. (The JWT itself carries `sub`, `email`, `roles` — see
> `pkg/auth/token.go:26` — but the router forwards only `sub`+`roles`; `email`
> for `me` comes from the DB.)

### 2. gRPC request (internal service-to-service)

```
caller service ──gRPC──> grpc.Server ──> handler ──> UseCase ──> outbound adapters ──> Postgres
                :50051    (server.go)    (*_handler.go)  (app)

Auth: handlers read x-user-id / x-user-roles from gRPC *metadata*
      (same keys as GraphQL). GetUser enforces self-or-admin from those claims
      (getuser_handler.go:17); no caller ⇒ PermissionDenied.
```

The 5 RPCs map 1:1 to use cases: `RegisterUser`, `LoginUser`, `RefreshToken`,
`VerifyToken`, `GetUser` (`proto/identity/v1/identity.proto`).

### 3. Registration writes user + event atomically (transactional outbox)

`registeruser` builds a `user_registered` event, then inside **one** Postgres
transaction inserts the user row **and** the outbox row together
(`registeruser.go:51`). Either both commit or both roll back — no event is ever
published for a user that didn't persist.

---

## Public API surface

### GraphQL (`schema.graphql`)

```graphql
type Mutation {
  registerUser(input: RegisterUserInput!): User!   # public
  loginUser(input: LoginUserInput!): AuthPayload!   # public
}
type Query {
  me: User!                                         # requires a valid JWT
}

type User { id: ID!  email: String!  roles: [String!]!  createdAt: Time! }
type AuthPayload { accessToken: String!  refreshToken: String!  accessTokenExpiresAt: Time! }
```

### gRPC (`identity.v1.IdentityService`)

`RegisterUser` · `LoginUser` · `RefreshToken` · `VerifyToken` · `GetUser`

### Validation & error mapping

- **Email**: trimmed + lowercased, RFC-5322 single address, ≤254 chars
  (`domain/email.go`).
- **Password**: 8–1024 characters (`domain/password.go`).
- **Login** never reveals whether an email exists — bad email *or* bad password
  both return the same `ErrInvalidCredentials` (`loginuser.go:44`).
- Domain error → wire mapping:
  - GraphQL `extensions.code`: `INVALID`, `NOT_FOUND`, `CONFLICT`,
    `UNAUTHORIZED`, `FORBIDDEN`, else `INTERNAL` (`graphql/errors.go`).
  - gRPC `status` codes via `grpc/errors.go`.

---

## Local testing

There are two ways to run it locally. Pick based on what you're testing.

### Ports

| Port | What | Notes |
|---|---|---|
| `4000` | Apollo Router `/graphql` + Sandbox | **the client entry point** |
| `8081` | identity HTTP: `/graphql`, `/.well-known/jwks.json`, `/healthz`, `/readyz` | the subgraph |
| `50051` | identity gRPC (+ reflection + health) | internal API |
| `5432` | Postgres (`identity_db`) | created automatically (`deploy/postgres/init`) |

### Mode A — full stack in Docker (test through the gateway)

This is the realistic path: client → router → subgraph, with real JWT validation.

```bash
# One-time: the router's JWT feature needs an Apollo license (free plan works).
# Put these in .env (gitignored) — see .env.example:
#   APOLLO_KEY=service:...
#   APOLLO_GRAPH_REF=your-graph@current

task up        # generates the dev signing key, brings up infra + identity + router,
               # runs migrations, and blocks until everything is healthy
```

Then hit the router at `http://localhost:4000/graphql` (see Postman/curl below).
`task down` tears the services down; `task infra:down` stops the data plane.

> Without `APOLLO_KEY`/`APOLLO_GRAPH_REF` the **router refuses to start** — JWT
> validation is a licensed GraphOS feature. The rest of the stack still runs;
> you can fall back to Mode B and test the subgraph directly.

### Mode B — service-only hot-reload loop (no router)

Best for iterating on the service itself. Runs the data plane in Docker but the
identity binary on your host with hot reload.

```bash
task infra:up            # Postgres, Redpanda, Redis, etc. (no app services)
task identity:migrate    # apply migrations to localhost:5432/identity_db (needs golang-migrate CLI)
task identity:run        # air hot-reload if installed, else `go run ./cmd/identity`
```

The service comes up on `:8081` (GraphQL/JWKS) and `:50051` (gRPC) with an
**ephemeral RSA signing key** (`main.go:220`) — fine standalone, but the `kid`
rotates every restart, so don't pair it with a long-lived router in this mode.
Because there's no router validating JWTs, test `me`/`GetUser` by spoofing the
principal header/metadata directly (examples below).

### Other useful tasks

```bash
task test                 # unit + adapter tests
task identity:test:integration   # testcontainers Postgres (build-tag gated)
task identity:graphql:gen # regenerate gqlgen code after editing schema.graphql
task identity:sqlc        # regenerate sqlc code after editing queries/migrations
task supergraph:compose   # recompose deploy/router/supergraph.graphql from subgraph schemas
```

---

## Testing endpoints with Postman

### A. GraphQL via the router (Mode A) — the realistic flow

1. **New → GraphQL request**, URL `http://localhost:4000/graphql`. Postman will
   introspect the schema (the router has introspection + a Sandbox enabled, so
   you can also just open `http://localhost:4000` in a browser).

2. **Register** (no auth):
   ```graphql
   mutation Register($input: RegisterUserInput!) {
     registerUser(input: $input) { id email roles createdAt }
   }
   ```
   Variables:
   ```json
   { "input": { "email": "alice@example.com", "password": "supersecret" } }
   ```

3. **Login** (no auth) — grab the `accessToken`:
   ```graphql
   mutation Login($input: LoginUserInput!) {
     loginUser(input: $input) { accessToken refreshToken accessTokenExpiresAt }
   }
   ```
   ```json
   { "input": { "email": "alice@example.com", "password": "supersecret" } }
   ```

4. **me** (auth required): open the request's **Authorization** tab → type
   **Bearer Token** → paste the `accessToken`. The router validates it and
   forwards your identity to the subgraph.
   ```graphql
   query Me { me { id email roles createdAt } }
   ```
   - Omitting the token (or sending an expired one) ⇒ **HTTP 401** from the
     router before the subgraph is touched.

### B. GraphQL straight against the subgraph (Mode B, bypass router)

The subgraph at `:8081/graphql` does **no** JWT validation — it trusts the
`x-user-id` header the router normally sets. So you can test `me` by spoofing it
(POST transport only; there's no GET playground on the subgraph):

- URL `http://localhost:8081/graphql`, **Headers** tab → `x-user-id:
  <a-real-user-uuid>`, then run the `me` query above.

### C. gRPC via Postman (the internal API)

The gRPC server has **server reflection enabled** (`main.go:137`), so Postman
discovers the methods for you:

1. **New → gRPC request**, server URL `localhost:50051`, leave TLS **off**
   (plaintext).
2. Select **"Using server reflection"** → the `identity.v1.IdentityService`
   methods populate.
3. Pick a method, fill the message, **Invoke**:
   - `RegisterUser` / `LoginUser`: `{ "email": "...", "password": "..." }`
   - `VerifyToken`: `{ "access_token": "<jwt>" }`
   - `GetUser`: `{ "user_id": "<uuid>" }` **plus** a request **Metadata** entry
     `x-user-id: <same-or-admin-uuid>` (it enforces self-or-admin; without it
     you'll get `PermissionDenied`).

---

## Equivalent CLI calls (curl / grpcurl)

```bash
# Register (via router)
curl -s http://localhost:4000/graphql -H 'content-type: application/json' -d '{
  "query":"mutation($i:RegisterUserInput!){registerUser(input:$i){id email roles createdAt}}",
  "variables":{"i":{"email":"alice@example.com","password":"supersecret"}}}'

# Login → capture the access token
ACCESS=$(curl -s http://localhost:4000/graphql -H 'content-type: application/json' -d '{
  "query":"mutation($i:LoginUserInput!){loginUser(input:$i){accessToken}}",
  "variables":{"i":{"email":"alice@example.com","password":"supersecret"}}}' \
  | jq -r '.data.loginUser.accessToken')

# me (authenticated, via router)
curl -s http://localhost:4000/graphql \
  -H 'content-type: application/json' -H "authorization: Bearer $ACCESS" \
  -d '{"query":"query{me{id email roles createdAt}}"}'

# me straight against the subgraph (Mode B: spoof the forwarded principal)
curl -s http://localhost:8081/graphql -H 'content-type: application/json' \
  -H 'x-user-id: <uuid>' -d '{"query":"query{me{id email roles createdAt}}"}'

# Inspect the public keys the router validates against
curl -s http://localhost:8081/.well-known/jwks.json | jq

# gRPC (reflection enabled)
grpcurl -plaintext localhost:50051 list
grpcurl -plaintext -d '{"email":"alice@example.com","password":"supersecret"}' \
  localhost:50051 identity.v1.IdentityService/LoginUser
grpcurl -plaintext -H 'x-user-id: <uuid>' -d '{"user_id":"<uuid>"}' \
  localhost:50051 identity.v1.IdentityService/GetUser

# Health
curl -s http://localhost:8081/healthz
grpcurl -plaintext localhost:50051 grpc.health.v1.Health/Check
```

---

## Gotchas

- **Router won't start without `APOLLO_KEY`/`APOLLO_GRAPH_REF`** — JWT auth is a
  licensed GraphOS feature (free plan suffices). Fall back to Mode B to test the
  subgraph in isolation.
- **Ephemeral vs durable signing key**: `task identity:run` (Mode B) mints an
  ephemeral key whose `kid` rotates per restart. `task up` (Mode A) uses the
  durable `deploy/identity/jwt-dev.pem` (generated by `task identity:keygen`) so
  the published JWKS and issued tokens stay stable behind the router.
- **`me` requires the router** (or a manually-set `x-user-id`): the subgraph
  never parses the JWT itself.
- **`GetUser` (gRPC) is self-or-admin**: it denies by default when no caller is
  forwarded in metadata.
