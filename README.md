<!-- markdownlint-disable -->
# BaseGO

Fully opinionated go project scaffolding. Built for hexagonal and code generation with OAPI.
Enforces heavy separation of concerns and modular project.
Victoria metrics for scrapping, alongside prometheus for metrics.

### overall structure in directories
```sh
.
├── bin # gitignored binary folder, where make build drops the binary
├── cmd
│   └── api # entrypoint and loadpoint of resources
└── internal
    ├── api # api area with handlers, middlewares and similars, touches ONLY the gateway
    │   └── openapi # oapi spec
    ├── config # general config struct, uses viper by default
    ├── logger # helper logger, injected into services to create tracer logs and spans
    ├── metrics # helper metrics scaffolding and injector, allowing to metrify endpoints operations and inject into structs
    ├── domain # shared entities, dto and data models live here
    │   ├── entity # entities in general, such as models, dtos and errors
    │   └── gateway # transational area, where the API touches. Interfaces that are exposed to the api live here
    ├── resource # reusable primitives, such as db connections, tools and similars
    │   └── database
    │       ├── memory
    │       └── mongo
    └── service # business rules, meaty part
        └── device
```

## Notes

Architecture must be preserved. though it's a lot of things to build upfront, if upheld, speeds up development.
Be it with or without AI.

## Disclaimer

It's built by me, for me. Use at your own will, submit PRs and change after forking.

## Expected flow

```sh
basego create # basic creation, generic and starting point
basego create file [path-to-openapi.yml] # will generate using the spec as a source of truth
basego help # basic help
```

## Expected stack

- Viper for config
- slog or zap for logging (need discussion)
- victoriametrics and prometheus for metrics
- tracing and spans by default on, with otel

## Conerstone

It's a deterministic tool that WILL generate the same output with the same input.
