-- 评测任务及逐题结果快照 / Evaluation runs and per-question result snapshots.
CREATE TABLE IF NOT EXISTS evaluation_runs (
    id VARCHAR(200) PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    dataset_id VARCHAR(100) NOT NULL,
    status INTEGER NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    heartbeat_at TIMESTAMPTZ NOT NULL,
    detail TEXT NOT NULL,
    summary TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_evaluation_runs_tenant_started ON evaluation_runs (tenant_id, started_at DESC);
