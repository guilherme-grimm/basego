# Go wiring is generated, not hand-owned

`cmd/api/deps.go` and `cmd/api/services.go` become `deps_gen.go` and `services_gen.go`, regenerated from the project's file tree: drivers under `internal/resource/*` and services under `internal/service/*`, read in sorted order so the same tree always produces the same wiring. `basego add` never parses or rewrites hand-written Go. The only hand-owned files it edits are `config/config.yaml` and `docker-compose.yml`, through comment-preserving YAML node edits.

A user who wants custom wiring takes over a generated file by removing its `// Code generated` header. From then on it is a hand-owned wiring file tracked for drift (ADR 0002). `add` never edits it, drifted or not, and prints the snippet to paste instead.

## Considered Options

- **Edit hand-owned Go wiring in place (AST surgery)**: rejected. It is fragile and grows complex quickly, and it is exactly where clobbering hand-written code is most likely.
- **Never edit, always print snippets**: rejected. Safe, but it reduces `add` to a code printer.

## Consequences

This changes what `basego create` emits today, which is a layout contract change. It is cheap before v1, with no outside users, and expensive after. A reader looking for "where is the wiring" now finds generated files; their header must say how to regenerate them and how to take them over.
