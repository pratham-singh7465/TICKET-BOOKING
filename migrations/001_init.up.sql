CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS shows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    price_paise BIGINT NOT NULL CHECK (price_paise >= 0),
    per_user_limit INT NOT NULL DEFAULT 4 CHECK (per_user_limit > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TYPE seat_status AS ENUM ('available', 'held', 'confirmed');

CREATE TABLE IF NOT EXISTS seats (
    show_id UUID NOT NULL REFERENCES shows(id) ON DELETE CASCADE,
    seat_code VARCHAR(10) NOT NULL,
    status seat_status NOT NULL DEFAULT 'available',
    user_id TEXT,
    reservation_id UUID,
    held_until TIMESTAMPTZ,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (show_id, seat_code)
);


CREATE TYPE reservation_status AS ENUM ('pending', 'booked', 'cancelled');

CREATE TABLE IF NOT EXISTS reservations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    show_id UUID NOT NULL REFERENCES shows(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    amount_paise BIGINT NOT NULL CHECK (amount_paise >= 0),
    status reservation_status NOT NULL DEFAULT 'pending',
    idempotency_key TEXT NOT NULL,
    request_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (idempotency_key)
);

CREATE TABLE IF NOT EXISTS reservation_seats (
    reservation_id UUID NOT NULL REFERENCES reservations(id) ON DELETE CASCADE,
    seat_code VARCHAR(10) NOT NULL,
    PRIMARY KEY (reservation_id, seat_code)
);

-- FAST PER USER LIMIT CHECKS PER SHOW

CREATE INDEX idx_seats_show_user_active ON seats (show_id, user_id) WHERE status = 'held' OR status = 'confirmed';

CREATE INDEX idx_seats_held_until ON seats (held_until) WHERE status = 'held' AND held_until IS NOT NULL;