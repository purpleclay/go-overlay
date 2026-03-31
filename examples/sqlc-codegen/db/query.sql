-- name: CountExcuses :one
SELECT COUNT(*) FROM excuses;

-- name: SeedExcuse :exec
INSERT INTO excuses (category, body) VALUES (?, ?);

-- name: RandomExcuse :one
SELECT * FROM excuses
WHERE (sqlc.narg('category') IS NULL OR category = sqlc.narg('category'))
ORDER BY RANDOM()
LIMIT 1;

-- name: IncrementTimesUsed :exec
UPDATE excuses SET times_used = times_used + 1 WHERE id = ?;
