CREATE TABLE stored_files (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    storage_key  VARCHAR(1000) NOT NULL UNIQUE,
    bucket       VARCHAR(255) NOT NULL,
    filename     VARCHAR(500) NOT NULL,
    content_type VARCHAR(200) NOT NULL,
    size_bytes   BIGINT NOT NULL,
    sha256       VARCHAR(64) NOT NULL,
    uploaded_by  VARCHAR(255) NOT NULL,
    entity_type  VARCHAR(100),
    entity_id    UUID,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_stored_files_entity ON stored_files(entity_type, entity_id);
