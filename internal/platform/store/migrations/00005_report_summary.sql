-- +goose Up
ALTER TABLE reports
    ADD COLUMN worst_severity text,
    ADD COLUMN cluster_name text,
    ADD CONSTRAINT ck_reports_worst_severity
        CHECK (worst_severity IS NULL OR worst_severity IN ('healthy', 'attention', 'critical')),
    ADD CONSTRAINT ck_reports_summary_pair
        CHECK ((worst_severity IS NULL) = (cluster_name IS NULL)),
    ADD CONSTRAINT ck_reports_summary_collected
        CHECK (worst_severity IS NULL OR status = 'collected');

-- +goose Down
ALTER TABLE reports
    DROP CONSTRAINT ck_reports_summary_collected,
    DROP CONSTRAINT ck_reports_summary_pair,
    DROP CONSTRAINT ck_reports_worst_severity,
    DROP COLUMN cluster_name,
    DROP COLUMN worst_severity;
