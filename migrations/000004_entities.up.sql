CREATE TABLE entitati (
    id                   UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tip                  VARCHAR(30) NOT NULL CHECK (tip IN (
                             'PERSOANA_FIZICA',
                             'PERSOANA_JURIDICA',
                             'INSTITUTIE_PUBLICA'
                         )),
    denumire             VARCHAR(500) NOT NULL,
    adresa               TEXT,
    localitate           VARCHAR(200),
    judet                VARCHAR(100),
    telefon              VARCHAR(50),
    email                VARCHAR(255),
    cnp                  VARCHAR(64),
    prenume              VARCHAR(200),
    data_nasterii        DATE,
    loc_nasterii         VARCHAR(200),
    cui                  VARCHAR(20),
    nr_reg_com           VARCHAR(30),
    reprezentant_legal   VARCHAR(300),
    forma_juridica       VARCHAR(100),
    cod_siruta           VARCHAR(10),
    nivel_institutie     VARCHAR(100),
    tip_institutie       VARCHAR(100),
    website              VARCHAR(500),
    delivery_participant_id VARCHAR(255),
    institution_id       UUID REFERENCES institutions(id),
    created_by           VARCHAR(255) NOT NULL,
    active               BOOLEAN NOT NULL DEFAULT TRUE,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_entitati_tip ON entitati(tip);
CREATE INDEX idx_entitati_cui ON entitati(cui) WHERE cui IS NOT NULL;
CREATE INDEX idx_entitati_cnp ON entitati(cnp) WHERE cnp IS NOT NULL;
CREATE INDEX idx_entitati_institution ON entitati(institution_id);
CREATE INDEX idx_entitati_search ON entitati USING gin(to_tsvector('romanian', denumire));
