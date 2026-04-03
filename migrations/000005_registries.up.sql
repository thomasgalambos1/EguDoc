CREATE TYPE tip_registru AS ENUM (
    'INTRARI',
    'IESIRI',
    'INTERN',
    'PETITII',
    'CONTRACTE',
    'DECIZII',
    'HOTARARI',
    'DISPOZITII',
    'GENERAL'
);

CREATE TABLE registre (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    institution_id   UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    compartiment_id  UUID REFERENCES compartimente(id),
    denumire         VARCHAR(300) NOT NULL,
    prefix           VARCHAR(20) NOT NULL,
    tip              tip_registru NOT NULL DEFAULT 'GENERAL',
    an               INTEGER NOT NULL,
    nr_curent        INTEGER NOT NULL DEFAULT 0,
    nr_urmator       INTEGER NOT NULL DEFAULT 1,
    data_reset       DATE,
    is_default       BOOLEAN NOT NULL DEFAULT FALSE,
    active           BOOLEAN NOT NULL DEFAULT TRUE,
    created_by       VARCHAR(255) NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(institution_id, prefix, an)
);

CREATE INDEX idx_registre_institution ON registre(institution_id, an) WHERE active = TRUE;

CREATE TABLE politici_retentie (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    institution_id   UUID REFERENCES institutions(id),
    tip_document     VARCHAR(100) NOT NULL,
    termen_ani       INTEGER NOT NULL DEFAULT 10,
    permanent        BOOLEAN NOT NULL DEFAULT FALSE,
    descriere        TEXT,
    UNIQUE(institution_id, tip_document)
);
