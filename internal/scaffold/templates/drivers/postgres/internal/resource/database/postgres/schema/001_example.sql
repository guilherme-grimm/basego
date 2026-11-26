-- Schema for the example slice. Applied by the postgres container on first
-- start (docker-entrypoint-initdb.d); swap in a migration tool when needed.
CREATE TABLE examples (
    id   TEXT PRIMARY KEY,
    name TEXT NOT NULL
);
