-- +goose Up
CREATE INDEX ix_reports_by_profile ON reports (profile_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX ix_reports_by_profile;
