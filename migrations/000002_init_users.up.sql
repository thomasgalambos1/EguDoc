CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    subject       VARCHAR(255) UNIQUE NOT NULL,
    email         VARCHAR(255) UNIQUE NOT NULL,
    phone         VARCHAR(50),
    prenume       VARCHAR(200),
    nume          VARCHAR(200),
    avatar_url    VARCHAR(500),
    active        BOOLEAN NOT NULL DEFAULT TRUE,
    last_login_at TIMESTAMPTZ,
    last_login_ip VARCHAR(45),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email);

CREATE TABLE institutions (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    cui                     VARCHAR(20) UNIQUE NOT NULL,
    denumire                VARCHAR(500) NOT NULL,
    tip                     VARCHAR(100) NOT NULL,
    adresa                  TEXT,
    localitate              VARCHAR(200),
    judet                   VARCHAR(100),
    cod_siruta              VARCHAR(10),
    telefon                 VARCHAR(50),
    email                   VARCHAR(255),
    website                 VARCHAR(500),
    delivery_participant_id VARCHAR(255),
    archive_account_id      VARCHAR(255),
    active                  BOOLEAN NOT NULL DEFAULT TRUE,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_institutions_cui ON institutions(cui);

CREATE TABLE compartimente (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    denumire       VARCHAR(300) NOT NULL,
    cod            VARCHAR(50),
    descriere      TEXT,
    parent_id      UUID REFERENCES compartimente(id),
    active         BOOLEAN NOT NULL DEFAULT TRUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(institution_id, cod)
);

CREATE INDEX idx_compartimente_institution ON compartimente(institution_id);

CREATE TABLE user_institution_memberships (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_subject    VARCHAR(255) NOT NULL,
    institution_id  UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    compartiment_id UUID REFERENCES compartimente(id),
    functie         VARCHAR(200),
    primary_member  BOOLEAN NOT NULL DEFAULT TRUE,
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_subject, institution_id)
);
