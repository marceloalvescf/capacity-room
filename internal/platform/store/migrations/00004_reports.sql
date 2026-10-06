-- +goose Up
CREATE TABLE reports (
    id uuid NOT NULL,
    owner_id uuid NOT NULL,
    profile_id text NOT NULL,
    profile_revision_id uuid NOT NULL,
    request jsonb NOT NULL,
    status text NOT NULL,
    phase text NOT NULL,
    attempt integer NOT NULL DEFAULT 0,
    failure_code text,
    collect_job_id bigint NOT NULL,
    created_at timestamptz NOT NULL,
    started_at timestamptz,
    collected_at timestamptz,
    completed_at timestamptz,
    expires_at timestamptz NOT NULL,
    expired_at timestamptz,
    updated_at timestamptz NOT NULL,
    CONSTRAINT pk_reports PRIMARY KEY (id),
    CONSTRAINT fk_reports_owner FOREIGN KEY (owner_id) REFERENCES users (id) ON DELETE RESTRICT,
    CONSTRAINT fk_reports_profile FOREIGN KEY (profile_id) REFERENCES profiles (id) ON DELETE RESTRICT,
    CONSTRAINT fk_reports_revision FOREIGN KEY (profile_revision_id) REFERENCES profile_revisions (id) ON DELETE RESTRICT,
    CONSTRAINT ck_reports_status CHECK (status IN ('queued', 'running', 'collected', 'failed')),
    CONSTRAINT ck_reports_phase CHECK (
        (status = 'queued' AND phase = 'waiting')
        OR (status = 'running' AND phase IN ('preflight', 'collecting', 'analyzing', 'snapshotting'))
        OR (status = 'collected' AND phase IN ('rendering', 'complete'))
        OR (status = 'failed' AND phase = 'complete')),
    CONSTRAINT ck_reports_failure CHECK ((status = 'failed') = (failure_code IS NOT NULL)),
    CONSTRAINT ck_reports_attempt CHECK (attempt >= 0)
);
CREATE INDEX ix_reports_active_by_profile ON reports (profile_id) WHERE status IN ('queued', 'running');
CREATE INDEX ix_reports_due_for_expiry ON reports (expires_at, id) WHERE expired_at IS NULL;
CREATE INDEX ix_reports_due_for_purge ON reports (expires_at, id) WHERE expired_at IS NOT NULL;

CREATE TABLE report_snapshots (
    report_id uuid NOT NULL,
    schema_version text NOT NULL,
    body bytea NOT NULL,
    sha256 text NOT NULL,
    size_bytes bigint NOT NULL,
    created_at timestamptz NOT NULL,
    CONSTRAINT pk_report_snapshots PRIMARY KEY (report_id),
    CONSTRAINT fk_report_snapshots_report FOREIGN KEY (report_id) REFERENCES reports (id) ON DELETE RESTRICT,
    CONSTRAINT ck_report_snapshots_sha256 CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_report_snapshots_size CHECK (size_bytes = octet_length(body) AND size_bytes <= 33554432)
);

CREATE TABLE report_pdfs (
    report_id uuid NOT NULL,
    language text NOT NULL,
    status text NOT NULL,
    failure_code text,
    job_id bigint NOT NULL,
    body bytea,
    sha256 text,
    size_bytes bigint,
    updated_at timestamptz NOT NULL,
    CONSTRAINT pk_report_pdfs PRIMARY KEY (report_id, language),
    CONSTRAINT fk_report_pdfs_report FOREIGN KEY (report_id) REFERENCES reports (id) ON DELETE RESTRICT,
    CONSTRAINT ck_report_pdfs_language CHECK (language IN ('pt-BR', 'en')),
    CONSTRAINT ck_report_pdfs_status CHECK (status IN ('pending', 'ready', 'failed')),
    CONSTRAINT ck_report_pdfs_ready CHECK ((status = 'ready') = (body IS NOT NULL AND sha256 IS NOT NULL AND size_bytes IS NOT NULL)),
    CONSTRAINT ck_report_pdfs_failure CHECK ((status = 'failed') = (failure_code IS NOT NULL)),
    CONSTRAINT ck_report_pdfs_sha256 CHECK (sha256 IS NULL OR sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_report_pdfs_size CHECK (size_bytes IS NULL OR (size_bytes = octet_length(body) AND size_bytes <= 67108864))
);

CREATE TABLE report_idempotency_keys (
    owner_id uuid NOT NULL,
    key text NOT NULL,
    request_sha256 text NOT NULL,
    report_id uuid NOT NULL,
    created_at timestamptz NOT NULL,
    CONSTRAINT pk_report_idempotency_keys PRIMARY KEY (owner_id, key),
    CONSTRAINT fk_report_idempotency_keys_owner FOREIGN KEY (owner_id) REFERENCES users (id) ON DELETE RESTRICT,
    CONSTRAINT fk_report_idempotency_keys_report FOREIGN KEY (report_id) REFERENCES reports (id) ON DELETE RESTRICT,
    CONSTRAINT uq_report_idempotency_keys_report UNIQUE (report_id)
);

-- +goose StatementBegin
CREATE FUNCTION reject_report_snapshot_update() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'report_snapshots rows are immutable' USING ERRCODE = 'integrity_constraint_violation';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER report_snapshots_are_immutable
    BEFORE UPDATE ON report_snapshots
    FOR EACH ROW EXECUTE FUNCTION reject_report_snapshot_update();

-- +goose Down
DROP TRIGGER report_snapshots_are_immutable ON report_snapshots;
DROP TABLE report_idempotency_keys;
DROP TABLE report_pdfs;
DROP TABLE report_snapshots;
DROP TABLE reports;
DROP FUNCTION reject_report_snapshot_update();
