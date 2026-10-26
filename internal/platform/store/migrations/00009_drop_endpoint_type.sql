-- +goose Up

-- Stored snapshots of schema 3.0 become schema 4.0: the schema version in the
-- first member turns to 4.0 and the one line that holds the endpoint type of
-- the report is removed. Encode writes two-space indented JSON, and a raw
-- newline cannot occur inside a JSON string, so the patterns are exact. The
-- digest and the size follow the new bytes. Only a snapshot that holds both
-- patterns is converted; any other stays at 3.0 for the check below. The
-- immutability trigger stays off for this one statement alone.
ALTER TABLE report_snapshots DISABLE TRIGGER report_snapshots_are_immutable;
UPDATE report_snapshots AS s
SET body = n.body,
    sha256 = encode(sha256(n.body), 'hex'),
    size_bytes = octet_length(n.body),
    schema_version = '4.0'
FROM (
    SELECT report_id,
           convert_to(
               regexp_replace(
                   regexp_replace(convert_from(body, 'UTF8'), '^\{\n  "schema_version": "3\.0",', E'{\n  "schema_version": "4.0",'),
                   '\n      "endpoint_type": "(prometheus|grafana)",', ''),
               'UTF8') AS body
    FROM report_snapshots
    WHERE schema_version = '3.0'
      AND convert_from(body, 'UTF8') ~ '^\{\n  "schema_version": "3\.0",'
      AND convert_from(body, 'UTF8') ~ '\n      "endpoint_type": "(prometheus|grafana)",'
) AS n
WHERE s.report_id = n.report_id;
ALTER TABLE report_snapshots ENABLE TRIGGER report_snapshots_are_immutable;

-- Stops the migration, and so rolls it back, when a snapshot is left at
-- schema 3.0 or still holds the endpoint type line, and names the report of
-- the first one it refuses.
-- +goose StatementBegin
DO $$
DECLARE
    bad_id uuid;
BEGIN
    SELECT report_id INTO bad_id
    FROM report_snapshots
    WHERE schema_version = '3.0'
       OR convert_from(body, 'UTF8') ~ '\n      "endpoint_type": '
    ORDER BY report_id
    LIMIT 1;
    IF bad_id IS NOT NULL THEN
        RAISE EXCEPTION 'the snapshot of report % cannot be carried to schema 4.0', bad_id;
    END IF;
END;
$$;
-- +goose StatementEnd

-- A snapshot of another schema is refused at insert, so a worker still
-- running the previous version cannot store one after this migration.
ALTER TABLE report_snapshots ADD CONSTRAINT report_snapshots_schema_version CHECK (schema_version = '4.0');

-- Profile revisions lose the endpoint type. The fingerprints stay as they
-- were computed when each revision was created.
ALTER TABLE profile_revisions DISABLE TRIGGER profile_revisions_are_immutable;
UPDATE profile_revisions
SET document = document - 'endpoint_type'
WHERE document ? 'endpoint_type';
ALTER TABLE profile_revisions ENABLE TRIGGER profile_revisions_are_immutable;

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    RAISE EXCEPTION 'this migration cannot be reverted';
END;
$$;
-- +goose StatementEnd
