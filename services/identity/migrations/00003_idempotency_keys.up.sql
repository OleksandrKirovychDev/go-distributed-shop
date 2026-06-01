CREATE TABLE idempotency_keys (
    key           UUID PRIMARY KEY,
    request_hash  TEXT NOT NULL,
    response_body BYTEA,
    status_code   INT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at    TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_idempotency_keys_expires_at ON idempotency_keys (expires_at);
