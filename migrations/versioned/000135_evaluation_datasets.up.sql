-- 可编辑草稿和不可变发布版本 / Editable drafts and immutable published versions.
CREATE TABLE IF NOT EXISTS evaluation_datasets (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    name VARCHAR(800) NOT NULL,
    description TEXT NOT NULL,
    draft_revision INTEGER NOT NULL,
    draft_content TEXT NOT NULL,
    draft_total INTEGER NOT NULL,
    latest_version INTEGER NOT NULL DEFAULT 0,
    published_hash VARCHAR(64) NOT NULL DEFAULT '',
    published_total INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_evaluation_datasets_tenant_updated ON evaluation_datasets (tenant_id, updated_at DESC);
CREATE TABLE IF NOT EXISTS evaluation_dataset_versions (
    dataset_id VARCHAR(36) NOT NULL REFERENCES evaluation_datasets(id),
    version INTEGER NOT NULL,
    tenant_id BIGINT NOT NULL,
    name VARCHAR(800) NOT NULL,
    description TEXT NOT NULL,
    content TEXT NOT NULL,
    content_hash VARCHAR(64) NOT NULL,
    total INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (dataset_id, version)
);
CREATE INDEX IF NOT EXISTS idx_evaluation_dataset_versions_tenant ON evaluation_dataset_versions (tenant_id, dataset_id);
