-- +goose Up
CREATE TABLE users (
    id            uuid        NOT NULL,
    username      text        NOT NULL,
    password_hash text        NOT NULL,
    role          text        NOT NULL,
    language      text        NOT NULL,
    active        boolean     NOT NULL,
    created_at    timestamptz NOT NULL,
    updated_at    timestamptz NOT NULL,
    CONSTRAINT pk_users PRIMARY KEY (id),
    CONSTRAINT uq_users_username UNIQUE (username),
    CONSTRAINT ck_users_role CHECK (role IN ('administrator', 'operator', 'reader')),
    CONSTRAINT ck_users_language CHECK (language IN ('pt-BR', 'en'))
);

CREATE TABLE sessions (
    id              uuid        NOT NULL,
    token_hash      bytea       NOT NULL,
    csrf_hash       bytea       NOT NULL,
    user_id         uuid        NOT NULL,
    created_at      timestamptz NOT NULL,
    expires_at      timestamptz NOT NULL,
    revoked_at      timestamptz,
    revoked_reason  text,
    rotated_from_id uuid,
    CONSTRAINT pk_sessions PRIMARY KEY (id),
    CONSTRAINT uq_sessions_token_hash UNIQUE (token_hash),
    CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE RESTRICT,
    CONSTRAINT fk_sessions_rotated_from FOREIGN KEY (rotated_from_id) REFERENCES sessions (id) ON DELETE RESTRICT,
    CONSTRAINT ck_sessions_revocation CHECK ((revoked_at IS NULL) = (revoked_reason IS NULL))
);
CREATE INDEX ix_sessions_active_by_user ON sessions (user_id) WHERE revoked_at IS NULL;

CREATE TABLE login_rate_limits (
    key_hash          bytea       NOT NULL,
    window_started_at timestamptz NOT NULL,
    failures          integer     NOT NULL,
    blocked_until     timestamptz,
    updated_at        timestamptz NOT NULL,
    CONSTRAINT pk_login_rate_limits PRIMARY KEY (key_hash)
);

CREATE TABLE audit_events (
    id          bigint      GENERATED ALWAYS AS IDENTITY,
    actor_id    uuid,
    occurred_at timestamptz NOT NULL,
    action      text        NOT NULL,
    target_type text,
    target_id   text,
    outcome     text        NOT NULL,
    metadata    jsonb       NOT NULL,
    CONSTRAINT pk_audit_events PRIMARY KEY (id),
    CONSTRAINT fk_audit_events_actor FOREIGN KEY (actor_id) REFERENCES users (id) ON DELETE RESTRICT,
    CONSTRAINT ck_audit_events_outcome CHECK (outcome IN ('succeeded', 'failed'))
);

-- +goose StatementBegin
CREATE FUNCTION reject_audit_event_mutation() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit events are append-only' USING ERRCODE = 'integrity_constraint_violation';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER audit_events_are_append_only
    BEFORE UPDATE OR DELETE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION reject_audit_event_mutation();

-- +goose Down
DROP TABLE audit_events;
DROP FUNCTION reject_audit_event_mutation();
DROP TABLE login_rate_limits;
DROP TABLE sessions;
DROP TABLE users;
