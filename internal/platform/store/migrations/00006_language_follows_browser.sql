-- +goose Up
ALTER TABLE users ADD COLUMN language_follows_browser boolean NOT NULL DEFAULT false;
ALTER TABLE users ALTER COLUMN language_follows_browser DROP DEFAULT;

-- +goose Down
ALTER TABLE users DROP COLUMN language_follows_browser;
