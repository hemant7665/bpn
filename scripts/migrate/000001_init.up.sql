CREATE SCHEMA IF NOT EXISTS write_model;

CREATE SCHEMA IF NOT EXISTS read_model;

CREATE TABLE IF NOT EXISTS write_model.users (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS read_model.users_summary (
    id INT PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);
