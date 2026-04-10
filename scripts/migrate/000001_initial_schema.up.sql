-- Single baseline: write_model.users + read_model.users_summary (materialized view).
-- If you previously applied older 000002–000003 files, reset the DB or fix schema_migrations before re-running.

CREATE SCHEMA IF NOT EXISTS write_model;
CREATE SCHEMA IF NOT EXISTS read_model;

CREATE TABLE write_model.users (
    id SERIAL PRIMARY KEY,
    tenant_id TEXT NOT NULL DEFAULT 'default-tenant',
    username TEXT NOT NULL,
    email TEXT NOT NULL,
    phone_no TEXT,
    date_of_birth DATE,
    gender TEXT,
    password_hash TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_users_tenant_email ON write_model.users (tenant_id, lower(email));
CREATE INDEX idx_users_username_lower ON write_model.users (lower(username));

CREATE MATERIALIZED VIEW read_model.users_summary AS
SELECT
    u.id,
    u.tenant_id,
    u.username,
    u.email,
    u.phone_no,
    u.date_of_birth,
    u.gender,
    u.created_at
FROM write_model.users u
WITH DATA;

CREATE UNIQUE INDEX idx_users_summary_id ON read_model.users_summary (id);
CREATE INDEX idx_users_summary_email_lower ON read_model.users_summary (lower(email));
CREATE INDEX idx_users_summary_username_lower ON read_model.users_summary (lower(username));
