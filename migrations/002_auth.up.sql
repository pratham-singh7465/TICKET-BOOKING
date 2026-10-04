-- Dev/test users and bearer API tokens (hash stored; plain token pattern documented in README).
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS api_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL,
    label TEXT NOT NULL DEFAULT '',
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT api_tokens_token_hash_key UNIQUE (token_hash)
);

CREATE INDEX IF NOT EXISTS idx_api_tokens_active_hash
    ON api_tokens (token_hash)
    WHERE revoked_at IS NULL;

INSERT INTO users (id) VALUES
    ('user1'),
    ('user2'),
    ('user3'),
    ('user4'),
    ('user5')
ON CONFLICT (id) DO NOTHING;

-- Plain bearer token for userN is: token-userN (e.g. token-user1).
INSERT INTO api_tokens (user_id, token_hash, label)
SELECT
    u.id,
    encode(digest('token-' || u.id, 'sha256'), 'hex'),
    'dev-' || u.id
FROM users u
WHERE u.id IN ('user1', 'user2', 'user3', 'user4', 'user5')
ON CONFLICT (token_hash) DO NOTHING;
