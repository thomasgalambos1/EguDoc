CREATE TABLE lant_aprobare (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    institution_id    UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    compartiment_id   UUID REFERENCES compartimente(id),
    tip_document      VARCHAR(100),
    approver_subject  VARCHAR(255),
    approver_role     VARCHAR(100),
    ordine            INTEGER NOT NULL DEFAULT 1,
    obligatoriu       BOOLEAN NOT NULL DEFAULT TRUE,
    active            BOOLEAN NOT NULL DEFAULT TRUE,
    UNIQUE(institution_id, compartiment_id, tip_document, ordine)
);

CREATE INDEX idx_lant_aprobare_compartiment ON lant_aprobare(compartiment_id, ordine) WHERE active = TRUE;
