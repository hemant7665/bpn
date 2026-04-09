-- Read-side materialized projection of write_model.import_jobs (refresh after writes; see ImportJobRepository).
CREATE MATERIALIZED VIEW read_model.import_jobs_summary AS
SELECT
    id,
    tenant_id,
    requested_by,
    status,
    csv_s3_key,
    report_s3_key,
    total_rows,
    passed_rows,
    failed_rows,
    error_message,
    created_at,
    updated_at
FROM write_model.import_jobs
WITH DATA;

CREATE UNIQUE INDEX idx_import_jobs_summary_id ON read_model.import_jobs_summary (id);

CREATE INDEX idx_import_jobs_summary_tenant_created
    ON read_model.import_jobs_summary (tenant_id, created_at DESC);

CREATE INDEX idx_import_jobs_summary_tenant_status
    ON read_model.import_jobs_summary (tenant_id, status);
