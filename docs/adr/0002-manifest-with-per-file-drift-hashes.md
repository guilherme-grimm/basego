# Manifest with per-file drift hashes, not a lockfile

`basego create` writes `.basego.yaml`, holding the basego version, the layout contract version, and one SHA-256 baseline hash per **taken-over** generated file. Hashes are taken over gofmt-normalised content. A project's current resources and drivers are read from the filesystem, never stored in the manifest, so the manifest cannot drift from reality when someone deletes a driver directory by hand.

The model follows sqlx's offline mode: the manifest is the baseline, `basego check` reports drift per file (optionally failing CI), and an explicit re-baseline step records intentional edits. Unlike sqlx, drift is not an error, because editing hand-owned code is legitimate in a project that owns itself. Instead, `basego add` stops editing a drifted file automatically and prints the snippet to paste.

The hand-owned YAML files `add` edits (`config/config.yaml`, `docker-compose.yml`) are **not** hashed. They are edited constantly by hand, so hashing them would make `add` fall back to snippets almost every time. They are protected by how they are edited: `add` only inserts keys that are missing, never modifies existing nodes, and refuses with a snippet when the key already exists in a different shape. Inserting a key cannot clobber a hand edit, so drift does not apply.

## Considered Options

- **Lockfile of resources/drivers/services**: rejected. It duplicates state that already lives in the file tree and goes stale on every hand edit.
- **One aggregate hash over "core modules"**: rejected. It says something changed but not which file, so `add` could not stay automatic for the untouched files.
- **Hashing the YAML wiring files too**: rejected (see above). Insert-only edits make it unnecessary, and it would block `add` on routine config edits.
- **SHA-1**: rejected in favour of SHA-256. The cost is identical, it is in the standard library, and SHA-1 draws "broken hash" review noise even though collision resistance is irrelevant for drift detection.

## Consequences

A freshly created project's manifest hashes nothing; entries appear only when a user takes over a generated file (ADR 0004). The hash is a convention, not a tamper-proof seal; anyone can recompute it. Generated files (`// Code generated … DO NOT EDIT.`) are outside the drift model: they are regenerated, and `check` may warn when one was edited, since regeneration will overwrite the edit. YAML node insertion can re-indent the surrounding block even though comments survive, so the insert path must be tested against hand-edited configs.
