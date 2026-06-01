-- name: InsertOutboxEvent :exec
INSERT INTO outbox (aggregate_id, topic, key, payload, headers)
VALUES ($1, $2, $3, $4, $5);
