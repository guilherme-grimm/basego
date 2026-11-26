# `basego add` operates only on basego projects

`basego add` extends projects created by `basego create` that still follow the layout contract. It never targets arbitrary Go modules. Every safety property of `add`, including idempotency, determinism and never clobbering hand-written code, depends on knowing where slices, wiring and resource directories live. An arbitrary repository offers none of that, and supporting it would turn basego into a general code-modification engine, which is a different product.

"Brownfield" therefore means a basego project that has evolved by hand since creation, not an unrelated codebase.
