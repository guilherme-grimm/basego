# Generators are pinned standalone tools, run via `go generate`

Every resource that `basego add` brings in is spec-first: a declarative spec lives in the project and code is generated from it. Where a mature generator exists (oapi-codegen, sqlc), basego only orchestrates it. Where none exists (e.g. kafka, nats, sqs, mongo), basego publishes its own generator as an importable, versioned Go package, invoked from `//go:generate go run github.com/guilherme-grimm/basego/<gen>@<version>`.

Generated projects therefore never need basego installed to regenerate code. `go generate` with pinned tools is the single mental model, whether the generator is third-party or basego's own, and it keeps DESIGN §1's promise that the tool walks away after scaffolding. Requiring basego at development time, or generating once and freezing the output, would contradict what basego preaches.

## Consequences

basego's generators and their spec formats become public, versioned API surface. A breaking change to a spec format needs a new major version of that generator, and existing projects keep working because they pin the version they were generated with.
