-- +goose Up

-- Stored snapshots of schema 2.0 become schema 3.0: the schema version in the
-- first member turns to 3.0 and the one line that holds the minimum severity
-- of the report is removed. Encode writes two-space indented JSON, and a raw
-- newline cannot occur inside a JSON string, so the pattern is exact. The
-- digest and the size follow the new bytes. The immutability trigger stays
-- off for this one statement alone.
ALTER TABLE report_snapshots DISABLE TRIGGER report_snapshots_are_immutable;
UPDATE report_snapshots AS s
SET body = n.body,
    sha256 = encode(sha256(n.body), 'hex'),
    size_bytes = octet_length(n.body),
    schema_version = '3.0'
FROM (
    SELECT report_id,
           convert_to(
               regexp_replace(
                   regexp_replace(convert_from(body, 'UTF8'), '^\{\n  "schema_version": "2\.0",', E'{\n  "schema_version": "3.0",'),
                   '\n    "min_severity": "(all|attention|critical)",', ''),
               'UTF8') AS body
    FROM report_snapshots
    WHERE schema_version = '2.0'
) AS n
WHERE s.report_id = n.report_id;
ALTER TABLE report_snapshots ENABLE TRIGGER report_snapshots_are_immutable;

-- Stops the migration, and so rolls it back, when a snapshot is left at
-- schema 2.0 or still holds the minimum severity line.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM report_snapshots
        WHERE schema_version = '2.0'
           OR convert_from(body, 'UTF8') ~ '\n    "min_severity": '
    ) THEN
        RAISE EXCEPTION 'a stored snapshot still holds the minimum severity';
    END IF;
END;
$$;
-- +goose StatementEnd

-- Profile revisions lose the two keys of the minimum severity. The
-- fingerprints stay as they were computed when each revision was created.
ALTER TABLE profile_revisions DISABLE TRIGGER profile_revisions_are_immutable;
UPDATE profile_revisions
SET document = document - 'default_min_severity' - 'permitted_min_severities'
WHERE document ?| array['default_min_severity', 'permitted_min_severities'];
ALTER TABLE profile_revisions ENABLE TRIGGER profile_revisions_are_immutable;

-- Stored requests lose the minimum severity.
UPDATE reports
SET request = request - 'min_severity'
WHERE request ? 'min_severity';

-- The worst severity of a collected report is worked out again from its
-- snapshot, as the report shows its findings now: the profile's floor only
-- drops healthy findings, so the worst is critical, else attention, else
-- healthy. A body that jsonb refuses (a \u0000 escape) or whose findings are not a list
-- leaves the stored value as it was.
-- +goose StatementBegin
DO $$
DECLARE
    item record;
    findings jsonb;
    worst text;
BEGIN
    FOR item IN
        SELECT r.id, s.body
        FROM reports AS r
        JOIN report_snapshots AS s ON s.report_id = r.id
        WHERE r.worst_severity IS NOT NULL
    LOOP
        BEGIN
            findings := (convert_from(item.body, 'UTF8')::jsonb) -> 'findings';
            SELECT CASE
                       WHEN bool_or(f ->> 'severity' = 'critical') THEN 'critical'
                       WHEN bool_or(f ->> 'severity' = 'attention') THEN 'attention'
                       ELSE 'healthy'
                   END
            INTO worst
            FROM jsonb_array_elements(findings) AS f;
            UPDATE reports SET worst_severity = coalesce(worst, 'healthy') WHERE id = item.id;
        EXCEPTION WHEN untranslatable_character OR invalid_text_representation OR invalid_parameter_value THEN
            CONTINUE;
        END;
    END LOOP;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    RAISE EXCEPTION 'this migration cannot be reverted';
END;
$$;
-- +goose StatementEnd
