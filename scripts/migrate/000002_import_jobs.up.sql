-- Status ACCEPTED: set after startImport enqueues to SQS (before worker claims the job).
CREATE TABLE IF NOT EXISTS write_model.import_jobs (
    id UUID PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    requested_by INTEGER NOT NULL REFERENCES write_model.users (id),
    status TEXT NOT NULL CHECK (status IN ('PENDING', 'ACCEPTED', 'PROCESSING', 'COMPLETED', 'FAILED')),
    csv_s3_key TEXT NOT NULL,
    report_s3_key TEXT,
    total_rows INTEGER,
    passed_rows INTEGER,
    failed_rows INTEGER,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_import_jobs_tenant_created
    ON write_model.import_jobs (tenant_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_import_jobs_tenant_status
    ON write_model.import_jobs (tenant_id, status);
