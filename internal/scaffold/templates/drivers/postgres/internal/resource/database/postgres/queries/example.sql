-- name: UpsertExample :exec
INSERT INTO examples (id, name)
VALUES ($1, $2)
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;

-- name: GetExample :one
SELECT id, name FROM examples WHERE id = $1;
