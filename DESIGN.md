<!-- markdownlint-disable -->
# basego — Design

Reference for v1 implementation. Every decision below is locked unless explicitly marked deferred.

---

## 1. Purpose

Opinionated, deterministic scaffolding tool for Go services built on hexagonal architecture with OAPI-driven codegen. Same input, same output. The tool walks away after scaffolding — generated projects own themselves from t=0.

Mental model: **go-blueprint–style**, one-shot scaffold. Not a continuously-regenerating codegen.

---

## 2. CLI shape

```
basego create <name> [extension ...]
basego version
```

- `create` is the main module; extensions follow the project name.
- `file <spec.yaml>` is the first extension — includes OAPI-derived vertical slices at creation time.
- Future extensions (`basego add service <name>`, etc.) are deferred but the architecture supports them additively.

Flags on `create`:

| Flag | Purpose | Default |
|---|---|---|
| `--module` | Go module path | `<name>` |
| `--db` | Comma list of DB drivers | `memory` always included; no other default |
| `--cache` | Cache drivers (future) | none for v1 |

`basego version` prints `basego <semver> (commit <short-hash>, built <date>)` — stamped via `-ldflags`.

---

## 3. Output project layout

Reference tree for `basego create my-app file spec.yaml --db=mongo,postgres --module=github.com/foo/my-app`, sample service `example`:

```
my-app/
├── .gitignore
├── .gitattributes              # generated files marked linguist-generated
├── .golangci.yml               # errorlint, testpackage, exhaustive enabled
├── Dockerfile                  # multi-stage → distroless
├── Makefile                    # build, test, run, lint, generate, verify-generated
├── README.md                   # templated
├── docker-compose.yml          # app + chosen resources + full obs stack
├── go.mod / go.sum
├── bin/                        # gitignored
├── config/
│   └── config.yaml             # Viper source of truth
├── cmd/
│   └── api/
│       ├── main.go             # config → setups → Deps → buildServices → serve
│       ├── config.go           # Viper bootstrap → Config struct
│       ├── deps.go             # type Deps struct
│       ├── logger.go           # slog + otel bridge
│       ├── metrics.go          # prom registry + vm push
│       ├── tracer.go           # otel tracer provider
│       ├── memory.go           # SetupMemory() *memory.Store
│       ├── mongo.go            # SetupMongo(cfg) (*mongo.Client, error)
│       ├── postgres.go         # SetupPostgres(cfg) (*pgxpool.Pool, error)
│       └── services.go         # buildServices(deps, cfg) — the only file that knows the wiring graph
├── internal/
│   ├── api/
│   │   ├── router.go
│   │   ├── health.go           # /healthz + /readyz
│   │   ├── middleware/
│   │   │   ├── recovery.go
│   │   │   ├── request_id.go
│   │   │   ├── tracing.go      # otelhttp
│   │   │   ├── logging.go      # slog request logger
│   │   │   └── metrics.go      # per-route prom histogram
│   │   └── openapi/
│   │       ├── spec.yaml
│   │       └── example/
│   │           ├── types_gen.go
│   │           ├── server_gen.go
│   │           ├── handler.go
│   │           └── doc.go      # //go:generate oapi-codegen ...
│   ├── domain/
│   │   ├── entity/
│   │   │   ├── code.go         # type Code string + closed enum constants
│   │   │   ├── error.go        # type Error + New(msg, metric string, code Code)
│   │   │   └── example/
│   │   │       ├── example.go  # domain model
│   │   │       ├── errors.go   # sentinels: ErrNotFound, ErrConflict, ...
│   │   │       └── dto.go      # per-slice translated DTOs when needed
│   │   └── gateway/
│   │       └── example/
│   │           └── gateway.go  # type Gateway interface
│   ├── service/
│   │   └── example/
│   │       ├── service.go
│   │       └── service_test.go # package example_test, table-driven, memory gateway
│   └── resource/
│       └── database/
│           ├── memory/
│           │   ├── store.go
│           │   └── example.go
│           ├── mongo/
│           │   ├── client.go   # near-empty; pool lifecycle in cmd/api/mongo.go
│           │   └── example.go
│           └── postgres/
│               ├── client.go
│               └── example.go
└── test/
    └── e2e/                    # build-tag-gated, hits real HTTP
```

### Layout invariants

- **Vertical slicing everywhere** in domain (`entity/<slice>`, `gateway/<slice>`, `service/<slice>`) and in resource impls (`<driver>/<slice>.go`) and in API (`openapi/<tag>/`).
- **No shared `entity/shared/` package.** Cross-tag schemas duplicate into each slice with per-slice translation. Hexagonal-pure; accepted boilerplate cost.
- **`cmd/api/<resource>.go`** owns connection lifecycle. Resource packages under `internal/resource/database/<driver>/` hold only gateway implementations.
- **`cmd/api/services.go`** is the *only* file that knows the wiring graph (service ↔ gateway ↔ resource). `main.go` is a dumb loader.
- **`internal/api/openapi/<tag>/`** holds generated code AND its handler in the same dir/package. `doc.go` carries the `//go:generate` directive.

---

## 4. Fixed stack

Non-swappable in v1:

| Concern | Choice | Rationale |
|---|---|---|
| Config | Viper | YAML source, env override possible |
| Logging | slog (stdlib) | Zero dep; otel bridge first-class. zap deferred as future feature flag. |
| Metrics | Prometheus client + VictoriaMetrics | Per README cornerstone |
| Tracing | OpenTelemetry | On by default |
| HTTP router | chi | net/http-flavored, plays clean with otelhttp/prom middleware |
| OAPI codegen | deepmap/oapi-codegen | Wrapped, pinned version. **Strict-server mode** used. |
| Error body | RFC 7807 Problem+JSON | Standard, oapi-codegen aware |

---

## 5. Resources & driver model

- **Memory driver always ships**, regardless of `--db` flags. First-class runtime option, not just a test artifact.
- **Resources are scaffold-time-additive AND runtime-toggleable.**
  - Scaffold-time: `--db=mongo,postgres` decides which driver dirs and `cmd/api/<driver>.go` files get scaffolded.
  - Runtime: `service.<name>.driver` in config picks which implementation is wired in `services.go`.
- **Wiring lives in `cmd/api/services.go`**, a plain switch per service:
  ```go
  var exampleGW gatewayexample.Gateway
  switch cfg.Service.Example.Driver {
  case "mongo":    exampleGW = mongoresource.NewExampleGateway(deps.Mongo)
  case "postgres": exampleGW = pgresource.NewExampleGateway(deps.Postgres)
  case "memory":   exampleGW = memresource.NewExampleGateway(deps.Memory)
  default:         return nil, fmt.Errorf("unknown driver %q for service example", cfg.Service.Example.Driver)
  }
  logger.Info("wired", "service", "example", "driver", cfg.Service.Example.Driver)
  ```
- **No DI framework.** No `init()` magic. No registry. Plain code.
- **Startup log line per service** documenting which driver was wired and why (config path).
- **`service.<name>.enabled: false`** skips route registration AND gateway wiring entirely for that service. Used for dark-launching.

### Gateway impl matrix

For N drivers × M services, each cell gets a file. Strategy:

- **Memory cells**: always real implementation.
- **Real-driver cells**: hybrid.
  - **CRUD-detectable operations** (the six combos: `GET /<plural>`, `GET /<plural>/{id}`, `POST /<plural>`, `PUT /<plural>/{id}`, `PATCH /<plural>/{id}`, `DELETE /<plural>/{id}`) → generated real impl.
  - **Non-CRUD operations** (custom verbs, RPC-style endpoints) → `panic("not implemented: see TODO")` stub.

---

## 6. OAPI spec → vertical slice mapping

- **One tag = one service.** Untagged operations → fail at scaffold time with a specific error message.
- **Schemas referenced by a single tag's operations** → entity in that slice.
- **Schemas referenced by multiple tags** → duplicated per slice as a translated DTO. No shared package.
- **oapi-codegen invocation**: one run per tag with `--include-tags <tag>` plus a per-tag output dir. N+1 invocations for N tags. Acceptable cost; result is clean slice ownership.
- **Generated files are committed.** `.gitattributes` marks them `linguist-generated=true` so GitHub collapses them in PR diffs. CI runs `make verify-generated` to fail on drift.

---

## 7. Error model

### Type shape

```go
// internal/domain/entity/code.go
type Code string
const (
    CodeNotFound         Code = "not_found"
    CodeConflict         Code = "conflict"
    CodeValidationFailed Code = "validation_failed"
    CodeUnauthorized     Code = "unauthorized"
    CodeForbidden        Code = "forbidden"
    CodeInternal         Code = "internal"
    // extended as needed; closed enum for exhaustive lint
)

// internal/domain/entity/error.go
type Error struct {
    msg, metric string
    code        Code
}
func (e *Error) Error() string  { return e.msg }
func (e *Error) Code() Code     { return e.code }
func (e *Error) Metric() string { return e.metric }
func New(msg, metric string, code Code) *Error { ... }
```

- **`msg`**: user-safe, short, contract-stable. Goes into the response body.
- **`metric`**: low-cardinality label for `errors_total{code, label, route, method}`. Prevents prom cardinality bombs.
- **`code`**: typed `Code` constant; compile-time validation against the closed enum.

### Per-slice sentinels

Each slice owns its sentinel set in `internal/domain/entity/<slice>/errors.go`:

```go
var (
    ErrNotFound = entity.New("example not found",        "example_not_found", entity.CodeNotFound)
    ErrConflict = entity.New("example already exists",   "example_conflict",  entity.CodeConflict)
)
```

### Validation errors

Single typed value (not a sentinel) carrying client-facing context:

```go
type ValidationError struct{ Field, Reason string }
func (v ValidationError) Error() string  { return fmt.Sprintf("%s: %s", v.Field, v.Reason) }
func (v ValidationError) Code() Code     { return entity.CodeValidationFailed }
func (v ValidationError) Metric() string { return "validation_failed" }
```

`Field` and `Reason` are *contract* data (the API client legitimately needs them). The invalid value the user submitted, the regex it failed — those go to traces/logs, never into the payload.

### Translation discipline

- **Gateway impl translates driver errors → entity sentinels** via a `translateError` helper at the top of each `<driver>/<slice>.go`. Service code never imports driver-specific errors.
- **No `%w` wrapping of infra noise into the error chain.** The error returned upward is the clean sentinel. Diagnostic context (driver error text, query, id) goes to slog + otel at the moment of translation. One log line per error per layer that has unique info.

### HTTP mapping

Centralized in `internal/api/middleware/errors.go`:

```go
var codeToStatus = map[entity.Code]int{
    entity.CodeNotFound:         404,
    entity.CodeConflict:         409,
    entity.CodeValidationFailed: 422,
    entity.CodeUnauthorized:     401,
    entity.CodeForbidden:        403,
    entity.CodeInternal:         500,
}
```

`exhaustive` lint enforces the map covers every `Code`. RFC 7807 body:

```json
{ "type": "about:blank", "title": "Not Found", "status": 404, "code": "not_found", "detail": "example not found" }
```

Validation errors extend with an `errors` array of `{field, reason}`.

---

## 8. Error bubble path

End-to-end:

```
1. Driver call fails
   internal/resource/database/mongo/example.go
   - mongo.ErrNoDocuments → translateError() → entity.example.ErrNotFound
   - Diagnostic log: slog.ErrorContext(ctx, "mongo get failed", "id", id, "err", originalErr)
   - Return entity.example.ErrNotFound (bare sentinel)

2. Service propagates
   internal/service/example/service.go
   - Optional debug log: slog.DebugContext(ctx, "get example", "err", err)
   - Pass-through unless adding business semantics (rare)

3. Handler returns
   internal/api/openapi/example/handler.go
   - func (h *Handler) GetExample(ctx, req) (GetExampleResponseObject, error)
   - return nil, err

4. oapi-codegen strict-server wrapper (generated)
   - Sees non-nil error, calls registered error hook.

5. Error hook (internal/api/middleware/errors.go) — single point
   - errors.As(err, &coded) → code, msg, metric
   - prometheus errors_total.WithLabelValues(coded.Code(), coded.Metric(), route, method).Inc()
   - status := codeToStatus[coded.Code()]
   - Write RFC 7807 body, set span error, slog the HTTP-level response.
```

### Logging discipline

| Layer | Logs |
|---|---|
| Gateway impl | Driver-level diagnostic with full context (id, query, originalErr) — *only* place that has these |
| Service | Optional `Debug` for dev experience; disabled in prod via log level |
| Error hook | HTTP-level response (status, code, request_id) |

No double-logging. Service only logs at info+ if it adds new semantic context (e.g., translating one entity error to another).

### Panic recovery

Separate `recovery` middleware. Catches panics, logs with full stack and span, returns 500 + `code: "internal"`. Out of the strict-server error path.

---

## 9. HTTP server lifecycle

`main.go` template:

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

cfg, err := config.Load()
...
srv := &http.Server{Addr: cfg.Server.Addr, Handler: router}
go func() { _ = srv.ListenAndServe() }()

<-ctx.Done()
logger.Info("shutting down")

shutCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
defer cancel()
_ = srv.Shutdown(shutCtx)   // stop accepting, drain inflight
closeResources(deps)         // close pools in reverse setup order
_ = tracer.Shutdown(shutCtx) // flush spans
```

- **Context propagation** through every layer (gateway methods take `ctx context.Context` as first arg, always).
- **Shutdown timeout** is config-driven (`server.shutdown_timeout`, default 30s).
- **Teardown order**: stop router → drain inflight requests → close DB pools → flush otel.

---

## 10. Middleware chain

Order, outermost first:

```
recovery → request_id → otel_tracing → logging → metrics → handler
```

- **recovery** outermost: catches panics from everything inside.
- **request_id** before tracing: assigns ID so trace + logs share it.
- **otel_tracing** before logging: log records inherit trace + span IDs.
- **metrics** innermost (of the middlewares): histograms measure handler latency, not middleware overhead.
- **Error hook** is *inside* the strict-server wrapper at handler return time, not a chi middleware.

---

## 11. Healthcheck endpoints

- **`/healthz`** (liveness): process alive. Always 200. Cheap.
- **`/readyz`** (readiness): dependencies reachable. 503 until every `cmd/api/<resource>.go` setup reports ready. Used by k8s for traffic gating.

Both registered outside the strict-server / OAPI handler tree — they're not part of the spec.

---

## 12. Testing

### Principles

**Blackbox always.** Tests verify behavior and contracts, never implementation. Code is a tool; we test the tool by using it, not by inspecting it.

### Mechanics

1. **Test files use `_test` package suffix** (`package example_test`). Enforced by `testpackage` lint.
2. **Tests construct via public `New()` with memory gateway** as a real implementation, not a helper. Memory driver = first-class production option; tests reuse it.
3. **Table-driven tests** as the canonical shape. Sample ships a table test.
4. **Integration tests** under `internal/resource/database/<driver>/<gateway>_test.go`, gated by `//go:build integration`. Use testcontainers-go. Separate CI job.
5. **E2E tests** under `test/e2e/`, build-tag-gated, spin the full stack.

### Mocks

**No mocks in v1.** Memory driver implementing the gateway contract is the test double. No `mockgen`, no `gomock`. Deferred for future revisit if a concrete need emerges.

---

## 13. Config schema

`config/config.yaml` at scaffold time:

```yaml
server:
  port: 8080
  shutdown_timeout: 30s

log:
  level: info
  json: true

otel:
  endpoint: localhost:4317
  service_name: my-app

resources:
  mongo:
    uri: mongodb://localhost:27017
    database: my-app
  postgres:
    dsn: postgres://user:pass@localhost:5432/my-app
  # memory: no config needed

service:
  example:
    enabled: true
    driver: memory   # mongo | postgres | memory
```

- **Booleans where binary** (`log.json: true`). No string enums when not needed.
- **`service.<name>.enabled: false`** skips routes + wiring entirely.
- **`service.<name>.driver`** drives the runtime switch in `services.go`.
- **No `.env.example`** ships. Viper handles env override natively if anyone needs it.

---

## 14. What ships beyond `.go` files

Default-on, no flags in v1:

- `Makefile` (`build`, `test`, `run`, `lint`, `generate`, `verify-generated`)
- `Dockerfile` (multi-stage → distroless)
- `docker-compose.yml` — app + chosen resources + **full LGTM obs stack** (Grafana + Loki + Tempo + Prometheus + otel-collector). Endpoints default to compose service names and are overridable via the `observability:` block in `config/config.yaml`, so pointing at a real backend is a config edit, not a compose rewrite. v1 app-side instrumentation is a single Prometheus `/metrics` endpoint; full otel deep-wire (logger/tracer/metrics middleware) is staged after. See PLAN Deliverables 9–10.
- `.golangci.yml` (`errorlint`, `testpackage`, `exhaustive`, others)
- `.gitignore`
- `.gitattributes` (linguist-generated for `*_gen.go`)
- `README.md` (templated with name + chosen flags)
- `config/config.yaml`

Deferred for post-v1:

- CI workflows (will become `--ci=github|gitlab|none`)
- Pre-commit hook configs
- Migrations tooling
- Auth/authz primitives
- License file
- k8s manifests

---

## 15. Scaffold-time side effects

In order, after files are written:

1. `gofmt -w .` (canonical formatting; helps determinism)
2. `go mod init <module>`
3. `go generate ./...` (only if `file` extension was used)
4. `go mod tidy`
5. `git init` + initial commit `"initial scaffold from basego v<ver>"`

No `--no-tidy` / `--no-git` escape flags in v1. Deferred.

---

## 16. Idempotency

`basego create <name>` **hard errors** if `<name>/` already exists. No `--force`. Error message:

> directory '<name>' already exists. basego create scaffolds new projects only. For adding to an existing project, see future 'basego add' subcommands.

---

## 17. Foundations for future `basego add`

Already baked in by the v1 architecture; no extra groundwork needed:

- Vertical slicing means new services are pure-add (new dirs, no shared-file edits).
- `cmd/api/services.go` is the one wiring file — `basego add service <name>` regenerates or appends to it cleanly.
- Per-driver gateway matrix means `basego add service <name>` knows exactly which files to scaffold (one per existing driver in `internal/resource/database/*`).
- Generated `services_gen.go` pattern (regenerated by `add`) is optional — manual edit also works since `add` runs at user request, not on a hot path.

---

## 18. Deferred

| Item | Why deferred |
|---|---|
| zap as logger | slog covers v1; add as feature flag later |
| `--no-tidy` / `--no-git` flags | Not needed for MVP |
| CI workflow scaffolds | Post-v1 |
| Pre-commit configs | Not opinionated enough yet |
| Auth/authz primitives | Out of scope |
| Migrations tooling | Out of scope |
| Mocks (mockgen/gomock) | Memory driver covers contract testing |
| Per-resource integration test scaffolds | Sample ships unit tests only; integration build-tag gated but tests not pre-written for v1 |
| `basego add` subcommand | Architecture supports it; not implemented in v1 |

---

## 19. Cornerstone

basego is deterministic. Same input, same output. Generated artifacts are auditable derivatives of the spec + flags. The tool's job is to drop a starting point and leave.
