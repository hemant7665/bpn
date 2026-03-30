-- Migration: 001_create_users
-- Run this against your PostgreSQL database (RDS or local)

CREATE SCHEMA IF NOT EXISTS write_model;

CREATE SCHEMA IF NOT EXISTS read_model;

CREATE TABLE IF NOT EXISTS write_model.users (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Materialized view: mirrors write_model.users
-- Refreshed by the sync worker after every write event.
-- Use REFRESH MATERIALIZED VIEW CONCURRENTLY for zero-downtime refresh (requires unique index below).
CREATE MATERIALIZED VIEW IF NOT EXISTS read_model.users_summary AS
SELECT id, name, email, created_at
FROM write_model.users
WITH
    DATA;

-- Required for REFRESH CONCURRENTLY (non-blocking refresh)
CREATE UNIQUE INDEX IF NOT EXISTS users_summary_id_idx ON read_model.users_summary (id);