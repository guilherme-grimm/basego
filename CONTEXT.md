# basego

basego is a lifecycle tool for Go services. It scaffolds new projects to a fixed layout contract and later extends those projects on explicit request.

## Language

**basego project**:
A Go project created by `basego create` that still follows basego's layout contract. It is the only kind of project `basego add` operates on.
_Avoid_: managed project, generated project (once it has been edited by hand)

**Layout contract**:
The fixed set of directory and wiring conventions every **basego project** follows, such as the vertical slices, the single wiring file and the per-driver resource directories.
_Avoid_: template structure, skeleton

**Greenfield**:
Starting a new **basego project** with `basego create`.

**Brownfield**:
Extending an existing **basego project** that may have evolved by hand since it was created. It never means an arbitrary Go repository.
_Avoid_: legacy, existing repo

**Resource**:
A piece of reusable infrastructure a **basego project** talks to, such as a database, message broker or queue. `basego add <driver>` adds resources only, never business logic.
_Avoid_: module (clashes with Go modules), plugin, integration

**Driver**:
The concrete technology behind a **resource**, such as postgres, mongo, kafka or sqs. A **basego project** can have several drivers for the same **resource kind**.
_Avoid_: backend, provider

**Resource kind**:
The family a **driver** belongs to, such as SQL database, document database or messaging. Drivers of one kind share generation tooling where it exists (all SQL drivers use sqlc).
_Avoid_: category, type

**Spec**:
The declarative input inside a **basego project** that generated code is derived from, such as an OpenAPI document, sqlc queries and schema, or a basego-defined spec for a driver without existing tooling. A spec is the source of truth; its generated code is never edited by hand.
_Avoid_: contract (reserved for **layout contract**), schema (too narrow)

**Generator**:
A pinned, standalone tool that turns a **spec** into code and runs via `go generate` without basego installed. It is either third-party (oapi-codegen, sqlc) or published by basego for drivers that lack one.
_Avoid_: codegen plugin, template

**Manifest**:
The `.basego.yaml` file that marks a project as a **basego project**; `add` refuses a project without one. It records the basego version, the **layout contract** version and a baseline hash per **taken-over** file.
_Avoid_: lockfile, state file

**Wiring file**:
A hand-owned file involved in connecting a new **resource**: `config/config.yaml` and `docker-compose.yml`, plus any **generated wiring** a user has **taken over**. `add` edits the YAML ones by inserting missing keys only, so they carry no hash and cannot **drift**. It never edits taken-over Go; it prints the snippet to paste instead.
_Avoid_: core module, glue

**Generated wiring**:
The Go code that connects **resources** to services, regenerated from the project's file tree and never edited by hand.
_Avoid_: glue code, bootstrap

**Take over**:
Removing the generated header from a generated file so it becomes hand-owned. A taken-over file stops being regenerated, becomes a **wiring file** and gets a baseline hash in the **manifest**.
_Avoid_: eject, fork

**Drift**:
A **taken-over** file whose content no longer matches its baseline hash in the **manifest**, meaning it was edited outside basego. Drift is reported by `basego check`, never treated as an error, and does not change what `add` does: `add` never edits a taken-over file either way.
_Avoid_: tampering, corruption

## Flagged ambiguities

- "Module" was used for what `add` brings in. Resolved: **resource**, because "module" means a Go module.
- "Contract" was used both for the **layout contract** and for generator input. Resolved: generator input is a **spec**.
- "Brownfield" was used loosely to mean any existing Go repository. Resolved: it means an evolved **basego project** only. Arbitrary Go modules are out of scope for `basego add`.

## Example dialogue

> **Dev:** Can I run `basego add` on our old payments service?
> **Domain expert:** Only if it's a **basego project**. `add` relies on the **layout contract**, so a repo basego didn't create has nothing for it to anchor to.
> **Dev:** It was created with basego, but we've rewritten half of it since.
> **Domain expert:** That's still **brownfield** and still in scope, as long as the **layout contract** holds.
> **Dev:** We **took over** `services_gen.go` and customised it. Will `basego add kafka` overwrite it?
> **Domain expert:** No. `add` never edits a taken-over file, so it prints the wiring for you to paste and leaves the file alone. Your edits show up as **drift** in `basego check`. The kafka **spec** and its generated code are new files, and the **generator** is pinned, so `go generate` keeps working without basego.
> **Dev:** And our heavily edited `config.yaml`?
> **Domain expert:** `add` only inserts the missing `resources.kafka` block, so your edits are safe without any hash.
