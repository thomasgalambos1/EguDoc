CREATE TABLE workflow_events (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    document_id     UUID NOT NULL REFERENCES documente(id),
    institution_id  UUID NOT NULL REFERENCES institutions(id),

    action          VARCHAR(100) NOT NULL,
    old_status      VARCHAR(50),
    new_status      VARCHAR(50) NOT NULL,

    actor_subject   VARCHAR(255) NOT NULL,
    actor_ip        VARCHAR(45),

    from_compartiment_id  UUID REFERENCES compartimente(id),
    to_compartiment_id    UUID REFERENCES compartimente(id),
    assigned_user_subject VARCHAR(255),

    motiv           TEXT,
    metadata        JSONB,

    visibility      VARCHAR(30) NOT NULL DEFAULT 'WORKFLOW_ONLY',

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION prevent_workflow_event_modification()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'workflow_events records are immutable';
END;
$$;

CREATE TRIGGER workflow_events_immutable
BEFORE UPDATE OR DELETE ON workflow_events
FOR EACH ROW EXECUTE FUNCTION prevent_workflow_event_modification();

CREATE OR REPLACE FUNCTION prevent_workflow_event_truncate()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'workflow_events table cannot be truncated';
END;
$$;

CREATE TRIGGER workflow_events_no_truncate
BEFORE TRUNCATE ON workflow_events
FOR EACH STATEMENT EXECUTE FUNCTION prevent_workflow_event_truncate();

CREATE INDEX idx_workflow_events_document ON workflow_events(document_id, created_at DESC);
CREATE INDEX idx_workflow_events_actor ON workflow_events(actor_subject, created_at DESC);
CREATE INDEX idx_workflow_events_institution ON workflow_events(institution_id, created_at DESC);
