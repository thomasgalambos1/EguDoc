CREATE TYPE document_status AS ENUM (
    'INREGISTRAT',
    'ALOCAT_COMPARTIMENT',
    'IN_LUCRU',
    'FLUX_APROBARE',
    'FINALIZAT',
    'ARHIVAT',
    'ANULAT'
);

CREATE TYPE tip_document AS ENUM (
    'INTRARE',
    'IESIRE',
    'INTERN',
    'PETITIE',
    'CONTRACT',
    'DECIZIE',
    'HOTARARE',
    'DISPOZITIE',
    'ADRESA',
    'NOTIFICARE',
    'RAPORT',
    'REFERAT',
    'ADEVERINTA',
    'CERTIFICAT',
    'AUTORIZATIE',
    'AVIZ'
);

CREATE TYPE clasificare_document AS ENUM (
    'PUBLIC',
    'INTERN',
    'CONFIDENTIAL',
    'SECRET'
);

CREATE TABLE documente (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    nr_inregistrare         VARCHAR(50) NOT NULL,
    registru_id             UUID NOT NULL REFERENCES registre(id),
    institution_id          UUID NOT NULL REFERENCES institutions(id),

    tip                     tip_document NOT NULL,
    status                  document_status NOT NULL DEFAULT 'INREGISTRAT',
    clasificare             clasificare_document NOT NULL DEFAULT 'PUBLIC',

    emitent_id              UUID REFERENCES entitati(id),
    destinatar_id           UUID REFERENCES entitati(id),
    emitent_intern_id       UUID REFERENCES compartimente(id),
    destinatar_intern_id    UUID REFERENCES compartimente(id),

    obiect                  TEXT NOT NULL,
    continut                TEXT,
    cuvinte_cheie           TEXT[],
    numar_file              INTEGER,

    compartiment_curent_id  UUID REFERENCES compartimente(id),
    user_curent_subject     VARCHAR(255),
    awaiting_approver_subject VARCHAR(255),

    data_inregistrare       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    data_document           DATE,
    data_termen             DATE,
    data_finalizare         TIMESTAMPTZ,
    data_arhivare           TIMESTAMPTZ,

    termen_pastrare_ani     INTEGER NOT NULL DEFAULT 10,
    archive_document_id     VARCHAR(255),
    archive_status          VARCHAR(30) DEFAULT 'NOT_ARCHIVED',

    delivery_message_id     VARCHAR(255),
    delivery_status         VARCHAR(30),

    workflow_locked_until   TIMESTAMPTZ,

    document_parinte_id     UUID REFERENCES documente(id),
    nr_document_extern      VARCHAR(100),

    motiv_anulare           TEXT,
    rejection_count         INTEGER NOT NULL DEFAULT 0,
    created_by              VARCHAR(255) NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_documente_institution ON documente(institution_id, status);
CREATE INDEX idx_documente_registru ON documente(registru_id);
CREATE INDEX idx_documente_nr ON documente(nr_inregistrare);
CREATE INDEX idx_documente_status ON documente(status, data_inregistrare DESC);
CREATE INDEX idx_documente_compartiment ON documente(compartiment_curent_id) WHERE status NOT IN ('FINALIZAT', 'ANULAT');
CREATE INDEX idx_documente_user ON documente(user_curent_subject) WHERE status = 'IN_LUCRU';
CREATE INDEX idx_documente_search ON documente USING gin(to_tsvector('romanian', obiect || ' ' || COALESCE(continut,'')));

CREATE TABLE atasamente (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    document_id     UUID NOT NULL REFERENCES documente(id) ON DELETE CASCADE,
    storage_key     VARCHAR(1000) NOT NULL,
    filename        VARCHAR(500) NOT NULL,
    content_type    VARCHAR(200) NOT NULL,
    size_bytes      BIGINT NOT NULL,
    sha256          VARCHAR(64) NOT NULL,
    description     TEXT,
    uploaded_by     VARCHAR(255) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_atasamente_document ON atasamente(document_id);
