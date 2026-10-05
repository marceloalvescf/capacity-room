-- +goose Up
CREATE TABLE profiles (
    id                  text        NOT NULL,
    current_revision_id uuid,
    created_at          timestamptz NOT NULL,
    updated_at          timestamptz NOT NULL,
    CONSTRAINT pk_profiles PRIMARY KEY (id),
    CONSTRAINT ck_profiles_id CHECK (id ~ '^[a-z0-9]([a-z0-9._-]{0,61}[a-z0-9])?$' AND id NOT LIKE '%..%')
);

CREATE TABLE profile_revisions (
    id              uuid        NOT NULL,
    profile_id      text        NOT NULL,
    revision        integer     NOT NULL,
    fingerprint     text        NOT NULL,
    display_name    text        NOT NULL,
    active          boolean     NOT NULL,
    document        jsonb       NOT NULL,
    credential_file text        NOT NULL,
    ca_bundle_file  text        NOT NULL,
    created_by      uuid,
    created_at      timestamptz NOT NULL,
    CONSTRAINT pk_profile_revisions PRIMARY KEY (id),
    CONSTRAINT fk_profile_revisions_profile FOREIGN KEY (profile_id) REFERENCES profiles (id) ON DELETE RESTRICT,
    CONSTRAINT fk_profile_revisions_created_by FOREIGN KEY (created_by) REFERENCES users (id) ON DELETE RESTRICT,
    CONSTRAINT uq_profile_revisions_profile_revision UNIQUE (profile_id, revision),
    CONSTRAINT uq_profile_revisions_profile_fingerprint UNIQUE (profile_id, fingerprint),
    CONSTRAINT ck_profile_revisions_revision CHECK (revision >= 1)
);

ALTER TABLE profiles ADD CONSTRAINT fk_profiles_current_revision
    FOREIGN KEY (current_revision_id) REFERENCES profile_revisions (id) ON DELETE RESTRICT
    DEFERRABLE INITIALLY DEFERRED;

-- +goose StatementBegin
CREATE FUNCTION reject_profile_revision_mutation() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'profile revisions are immutable' USING ERRCODE = 'integrity_constraint_violation';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER profile_revisions_are_immutable
    BEFORE UPDATE OR DELETE ON profile_revisions
    FOR EACH ROW EXECUTE FUNCTION reject_profile_revision_mutation();
CREATE TRIGGER profile_revisions_are_not_truncated
    BEFORE TRUNCATE ON profile_revisions
    FOR EACH STATEMENT EXECUTE FUNCTION reject_profile_revision_mutation();

-- +goose StatementBegin
CREATE FUNCTION check_profile_pointer() RETURNS trigger AS $$
BEGIN
    IF NEW.current_revision_id IS NOT NULL THEN
        PERFORM 1 FROM profile_revisions WHERE id = NEW.current_revision_id AND profile_id = NEW.id;
        IF NOT FOUND THEN
            RAISE EXCEPTION 'current_revision_id must name a revision of the same profile' USING ERRCODE = 'integrity_constraint_violation';
        END IF;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE CONSTRAINT TRIGGER profile_pointer_matches_its_revision
    AFTER INSERT OR UPDATE ON profiles
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION check_profile_pointer();

-- +goose Down
ALTER TABLE profiles DROP CONSTRAINT fk_profiles_current_revision;
DROP TABLE profile_revisions;
DROP TABLE profiles;
DROP FUNCTION reject_profile_revision_mutation();
DROP FUNCTION check_profile_pointer();
