# EguDoc — Sub-plan B: Registratura & Workflow Engine

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended). Depends on Sub-plan A (Foundation) being complete.

**Goal:** Implement the full electronic document registry (registru electronic de intrări/ieșiri) per Romanian law and a generic workflow engine that routes documents through institution departments, between institutions, and between institutions and citizens or external companies.

**Architecture:** Three distinct layers: (1) Registratura — manages document registration, numbering, and the entity model (persons, companies, institutions); (2) Workflow Engine — a pure state-machine service that is agnostic to document type, operating on transition rules stored in the DB; (3) Approval Chains — configurable per compartiment for routing within and across departments. All workflow transitions are immutably logged in `workflow_events`.

**Tech Stack:** Go 1.24, pgx/v5, Chi v5 (handlers mounted on existing router from Sub-plan A)

---

## File Map

```
internal/
├── entities/
│   ├── model.go          # PersoanaFizica, PersoanaJuridica, InstitutiePublica + Entitate union
│   ├── service.go        # CRUD for all entity types
│   └── handler.go        # REST: POST/GET/PUT /api/entities
├── registry/
│   ├── model.go          # Registru, TipRegistru, RetentionPolicy
│   ├── service.go        # Registry CRUD + document number generation (thread-safe)
│   └── handler.go        # REST: /api/registries
├── registratura/
│   ├── model.go          # Document, TipDocument, DocumentStatus, Attachment
│   ├── service.go        # Document CRUD, search, file upload
│   ├── handler.go        # REST: /api/documents
│   └── number.go         # DocumentNumberGenerator (atomic, annual reset)
├── workflow/
│   ├── model.go          # WorkflowDefinition, WorkflowInstance, Transition, Step
│   ├── engine.go         # WorkflowEngine: Advance(), Validate(), GetAvailableActions()
│   ├── service.go        # WorkflowService: CreateInstance(), GetInstance(), ActOn()
│   ├── handler.go        # REST: /api/workflows/{id}/actions
│   └── audit.go          # WorkflowAuditService: immutable event log
└── approval/
    ├── model.go           # ApprovalChain, ApprovalStep
    └── service.go         # ApprovalService: GetChain(), NextApprover()

migrations/
├── 000004_entities.sql
├── 000005_registries.sql
├── 000006_documents.sql
├── 000007_workflow.sql
└── 000008_approval_chains.sql
```

---

## Task 11: Entity model migrations and service

**Files:**
- Create: `migrations/000004_entities.sql`
- Create: `internal/entities/model.go`
- Create: `internal/entities/service.go`
- Create: `internal/entities/handler.go`

- [ ] **Step 11.1: Write 000004_entities.sql**

```sql
-- migrations/000004_entities.sql
-- +migrate Up

-- Single table inheritance for all external parties:
-- citizens (persoane fizice), companies (persoane juridice), public institutions
CREATE TABLE entitati (
    id                   UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tip                  VARCHAR(30) NOT NULL CHECK (tip IN (
                             'PERSOANA_FIZICA',
                             'PERSOANA_JURIDICA',
                             'INSTITUTIE_PUBLICA'
                         )),
    -- Common fields
    denumire             VARCHAR(500) NOT NULL,   -- full name or company name
    adresa               TEXT,
    localitate           VARCHAR(200),
    judet                VARCHAR(100),
    telefon              VARCHAR(50),
    email                VARCHAR(255),
    -- Persoana fizica specific
    cnp                  VARCHAR(13),             -- hashed in app, stored as SHA-256 hex
    prenume              VARCHAR(200),
    data_nasterii        DATE,
    loc_nasterii         VARCHAR(200),
    -- Persoana juridica specific
    cui                  VARCHAR(20),
    nr_reg_com           VARCHAR(30),
    reprezentant_legal   VARCHAR(300),
    forma_juridica       VARCHAR(100),
    -- Institutie publica specific
    cod_siruta           VARCHAR(10),
    nivel_institutie     VARCHAR(100),             -- national, judetean, local
    tip_institutie       VARCHAR(100),             -- primarie, consiliu, minister, etc.
    website              VARCHAR(500),
    -- eDelivery participant ID (if institution is registered in eDelivery network)
    delivery_participant_id VARCHAR(255),
    -- Audit
    institution_id       UUID REFERENCES institutions(id),  -- owning institution (data controller)
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

-- +migrate Down
DROP TABLE IF EXISTS entitati;
```

- [ ] **Step 11.2: Write entities/model.go**

```go
// internal/entities/model.go
package entities

import (
	"time"
	"github.com/google/uuid"
)

type TipEntitate string

const (
	TipPersoanaFizica    TipEntitate = "PERSOANA_FIZICA"
	TipPersoanaJuridica  TipEntitate = "PERSOANA_JURIDICA"
	TipInstitutiePublica TipEntitate = "INSTITUTIE_PUBLICA"
)

// Entitate represents any external party that can be a sender or recipient of documents.
type Entitate struct {
	ID        uuid.UUID   `json:"id"`
	Tip       TipEntitate `json:"tip"`
	Denumire  string      `json:"denumire"`
	Adresa    string      `json:"adresa,omitempty"`
	Localitate string     `json:"localitate,omitempty"`
	Judet     string      `json:"judet,omitempty"`
	Telefon   string      `json:"telefon,omitempty"`
	Email     string      `json:"email,omitempty"`

	// Persoana fizica
	CNP          string     `json:"cnp,omitempty"`   // hashed
	Prenume      string     `json:"prenume,omitempty"`
	DataNasterii *time.Time `json:"data_nasterii,omitempty"`
	LocNasterii  string     `json:"loc_nasterii,omitempty"`

	// Persoana juridica
	CUI               string `json:"cui,omitempty"`
	NrRegCom          string `json:"nr_reg_com,omitempty"`
	ReprezentantLegal string `json:"reprezentant_legal,omitempty"`
	FormaJuridica     string `json:"forma_juridica,omitempty"`

	// Institutie publica
	CodSiruta                 string `json:"cod_siruta,omitempty"`
	NivelInstitutie           string `json:"nivel_institutie,omitempty"`
	TipInstitutie             string `json:"tip_institutie,omitempty"`
	Website                   string `json:"website,omitempty"`
	DeliveryParticipantID     string `json:"delivery_participant_id,omitempty"`

	InstitutionID *uuid.UUID `json:"institution_id,omitempty"`
	CreatedBy     string     `json:"created_by"`
	Active        bool       `json:"active"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type CreateEntitateDTO struct {
	Tip       TipEntitate `json:"tip" validate:"required"`
	Denumire  string      `json:"denumire" validate:"required,min=2,max=500"`
	Adresa    string      `json:"adresa"`
	Localitate string     `json:"localitate"`
	Judet     string      `json:"judet"`
	Telefon   string      `json:"telefon"`
	Email     string      `json:"email"`
	// Persoana fizica
	CNP         string     `json:"cnp"`
	Prenume     string     `json:"prenume"`
	// Persoana juridica
	CUI          string `json:"cui"`
	NrRegCom     string `json:"nr_reg_com"`
	// Institutie
	CodSiruta    string `json:"cod_siruta"`
	TipInstitutie string `json:"tip_institutie"`
}

type ListEntitatiParams struct {
	Search        string
	Tip           TipEntitate
	InstitutionID *uuid.UUID
	Page          int
	Limit         int
}
```

- [ ] **Step 11.3: Write entities/service.go**

```go
// internal/entities/service.go
package entities

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

func (s *Service) Create(ctx context.Context, dto CreateEntitateDTO, createdBy string, institutionID *uuid.UUID) (*Entitate, error) {
	cnpHashed := ""
	if dto.CNP != "" {
		h := sha256.Sum256([]byte(strings.TrimSpace(dto.CNP)))
		cnpHashed = hex.EncodeToString(h[:])
	}

	var e Entitate
	err := s.db.QueryRow(ctx, `
		INSERT INTO entitati
		  (tip, denumire, adresa, localitate, judet, telefon, email,
		   cnp, prenume, cui, nr_reg_com,
		   cod_siruta, tip_institutie,
		   institution_id, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		RETURNING id, tip, denumire, adresa, localitate, judet, telefon, email,
		          cnp, prenume, cui, nr_reg_com, cod_siruta, tip_institutie,
		          institution_id, created_by, active, created_at, updated_at
	`,
		dto.Tip, dto.Denumire, dto.Adresa, dto.Localitate, dto.Judet,
		dto.Telefon, dto.Email,
		cnpHashed, dto.Prenume, dto.CUI, dto.NrRegCom,
		dto.CodSiruta, dto.TipInstitutie,
		institutionID, createdBy,
	).Scan(
		&e.ID, &e.Tip, &e.Denumire, &e.Adresa, &e.Localitate, &e.Judet, &e.Telefon, &e.Email,
		&e.CNP, &e.Prenume, &e.CUI, &e.NrRegCom, &e.CodSiruta, &e.TipInstitutie,
		&e.InstitutionID, &e.CreatedBy, &e.Active, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create entitate: %w", err)
	}
	return &e, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Entitate, error) {
	var e Entitate
	err := s.db.QueryRow(ctx, `
		SELECT id, tip, denumire, adresa, localitate, judet, telefon, email,
		       cnp, prenume, cui, nr_reg_com, cod_siruta, tip_institutie,
		       delivery_participant_id, institution_id, created_by, active, created_at, updated_at
		FROM entitati WHERE id = $1 AND active = TRUE
	`, id).Scan(
		&e.ID, &e.Tip, &e.Denumire, &e.Adresa, &e.Localitate, &e.Judet, &e.Telefon, &e.Email,
		&e.CNP, &e.Prenume, &e.CUI, &e.NrRegCom, &e.CodSiruta, &e.TipInstitutie,
		&e.DeliveryParticipantID, &e.InstitutionID, &e.CreatedBy, &e.Active, &e.CreatedAt, &e.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get entitate: %w", err)
	}
	return &e, nil
}

func (s *Service) List(ctx context.Context, params ListEntitatiParams) ([]*Entitate, int, error) {
	if params.Page < 1 { params.Page = 1 }
	if params.Limit < 1 || params.Limit > 100 { params.Limit = 20 }
	offset := (params.Page - 1) * params.Limit

	conditions := []string{"active = TRUE"}
	args := []any{}
	i := 1

	if params.Tip != "" {
		conditions = append(conditions, fmt.Sprintf("tip = $%d", i))
		args = append(args, params.Tip); i++
	}
	if params.InstitutionID != nil {
		conditions = append(conditions, fmt.Sprintf("institution_id = $%d", i))
		args = append(args, params.InstitutionID); i++
	}
	if params.Search != "" {
		conditions = append(conditions, fmt.Sprintf(
			"(denumire ILIKE $%d OR cui ILIKE $%d OR email ILIKE $%d)", i, i+1, i+2,
		))
		q := "%" + params.Search + "%"
		args = append(args, q, q, q); i += 3
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	// Count
	var total int
	err := s.db.QueryRow(ctx, "SELECT COUNT(*) FROM entitati "+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count entitati: %w", err)
	}

	args = append(args, params.Limit, offset)
	query := fmt.Sprintf(`
		SELECT id, tip, denumire, adresa, localitate, judet, telefon, email,
		       cnp, prenume, cui, nr_reg_com, delivery_participant_id,
		       institution_id, created_by, active, created_at, updated_at
		FROM entitati %s
		ORDER BY denumire ASC
		LIMIT $%d OFFSET $%d
	`, where, i, i+1)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list entitati: %w", err)
	}
	defer rows.Close()

	var result []*Entitate
	for rows.Next() {
		var e Entitate
		if err := rows.Scan(
			&e.ID, &e.Tip, &e.Denumire, &e.Adresa, &e.Localitate, &e.Judet, &e.Telefon, &e.Email,
			&e.CNP, &e.Prenume, &e.CUI, &e.NrRegCom, &e.DeliveryParticipantID,
			&e.InstitutionID, &e.CreatedBy, &e.Active, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		result = append(result, &e)
	}
	return result, total, rows.Err()
}
```

- [ ] **Step 11.4: Commit**

```bash
git add migrations/000004_entities.sql internal/entities/
git commit -m "feat: add entitati (persons, companies, institutions) model and service"
```

---

## Task 12: Registry management and document numbering

**Files:**
- Create: `migrations/000005_registries.sql`
- Create: `internal/registry/model.go`
- Create: `internal/registry/service.go`
- Create: `internal/registry/number.go`

- [ ] **Step 12.1: Write 000005_registries.sql**

```sql
-- migrations/000005_registries.sql
-- +migrate Up

CREATE TYPE tip_registru AS ENUM (
    'INTRARI',          -- Registru intrari (incoming documents)
    'IESIRI',           -- Registru iesiri (outgoing documents)
    'INTERN',           -- Internal documents
    'PETITII',          -- Citizen petitions
    'CONTRACTE',        -- Contracts
    'DECIZII',          -- Decisions/orders
    'HOTARARI',         -- Council decisions
    'DISPOZITII',       -- Mayor dispositions
    'GENERAL'           -- General purpose registry
);

CREATE TABLE registre (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    institution_id   UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    compartiment_id  UUID REFERENCES compartimente(id),  -- NULL = institution-wide
    denumire         VARCHAR(300) NOT NULL,
    prefix           VARCHAR(20) NOT NULL,   -- e.g. "INT/", "IEST/", "CTR/"
    tip              tip_registru NOT NULL DEFAULT 'GENERAL',
    an               INTEGER NOT NULL,       -- year this registry belongs to
    nr_curent        INTEGER NOT NULL DEFAULT 0,
    nr_urmator       INTEGER NOT NULL DEFAULT 1,   -- pre-calculated next (thread safety)
    data_reset       DATE,                  -- when to reset numbering (Jan 1 each year)
    is_default       BOOLEAN NOT NULL DEFAULT FALSE,
    active           BOOLEAN NOT NULL DEFAULT TRUE,
    created_by       VARCHAR(255) NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(institution_id, prefix, an)
);

CREATE INDEX idx_registre_institution ON registre(institution_id, an) WHERE active = TRUE;

-- Document retention policies per document type
CREATE TABLE politici_retentie (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    institution_id   UUID REFERENCES institutions(id),  -- NULL = global policy
    tip_document     VARCHAR(100) NOT NULL,
    termen_ani       INTEGER NOT NULL DEFAULT 10,       -- retention in years
    permanent        BOOLEAN NOT NULL DEFAULT FALSE,    -- transfer to Arhivele Nationale
    descriere        TEXT,
    UNIQUE(institution_id, tip_document)
);

-- +migrate Down
DROP TABLE IF EXISTS politici_retentie;
DROP TABLE IF EXISTS registre;
DROP TYPE IF EXISTS tip_registru;
```

- [ ] **Step 12.2: Write registry/model.go**

```go
// internal/registry/model.go
package registry

import (
	"time"
	"github.com/google/uuid"
)

type TipRegistru string

const (
	TipIntrari     TipRegistru = "INTRARI"
	TipIesiri      TipRegistru = "IESIRI"
	TipIntern      TipRegistru = "INTERN"
	TipPetitii     TipRegistru = "PETITII"
	TipContracte   TipRegistru = "CONTRACTE"
	TipDecizii     TipRegistru = "DECIZII"
	TipHotarari    TipRegistru = "HOTARARI"
	TipDispozitii  TipRegistru = "DISPOZITII"
	TipGeneral     TipRegistru = "GENERAL"
)

type Registru struct {
	ID             uuid.UUID   `json:"id"`
	InstitutionID  uuid.UUID   `json:"institution_id"`
	CompartimentID *uuid.UUID  `json:"compartiment_id,omitempty"`
	Denumire       string      `json:"denumire"`
	Prefix         string      `json:"prefix"`
	Tip            TipRegistru `json:"tip"`
	An             int         `json:"an"`
	NrCurent       int         `json:"nr_curent"`
	IsDefault      bool        `json:"is_default"`
	Active         bool        `json:"active"`
	CreatedAt      time.Time   `json:"created_at"`
}

type RetentionPolicy struct {
	ID            uuid.UUID  `json:"id"`
	InstitutionID *uuid.UUID `json:"institution_id,omitempty"`
	TipDocument   string     `json:"tip_document"`
	TermenAni     int        `json:"termen_ani"`
	Permanent     bool       `json:"permanent"`
	Descriere     string     `json:"descriere,omitempty"`
}
```

- [ ] **Step 12.3: Write registry/number.go**

```go
// internal/registry/number.go
package registry

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GenerateNextNumber atomically allocates the next document number for a registry.
// Uses SELECT FOR UPDATE to prevent race conditions.
// Returns the formatted document number: "{prefix}{NNNNNN}" (zero-padded to 6 digits).
func GenerateNextNumber(ctx context.Context, pool *pgxpool.Pool, registryID uuid.UUID) (string, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var prefix string
	var nrUrmator int
	var an int
	var dataReset *time.Time

	err = tx.QueryRow(ctx, `
		SELECT prefix, nr_urmator, an, data_reset
		FROM registre
		WHERE id = $1 AND active = TRUE
		FOR UPDATE
	`, registryID).Scan(&prefix, &nrUrmator, &an, &dataReset)
	if err == pgx.ErrNoRows {
		return "", fmt.Errorf("registry not found or inactive")
	}
	if err != nil {
		return "", fmt.Errorf("lock registry: %w", err)
	}

	// Check for annual reset
	now := time.Now()
	if dataReset != nil && !dataReset.IsZero() && now.After(*dataReset) && now.Year() > an {
		// Reset: new year, start from 1
		nrUrmator = 1
		an = now.Year()
		nextReset := time.Date(now.Year()+1, 1, 1, 0, 0, 0, 0, time.UTC)
		dataReset = &nextReset
	}

	assigned := nrUrmator
	nextNum := nrUrmator + 1

	_, err = tx.Exec(ctx, `
		UPDATE registre
		SET nr_curent = $1, nr_urmator = $2, an = $3, data_reset = $4, updated_at = NOW()
		WHERE id = $5
	`, assigned, nextNum, an, dataReset, registryID)
	if err != nil {
		return "", fmt.Errorf("update registry counter: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit number: %w", err)
	}

	// Format: prefix + zero-padded 6-digit number
	return fmt.Sprintf("%s%06d", prefix, assigned), nil
}
```

- [ ] **Step 12.4: Write number generator test**

```go
// internal/registry/number_test.go
package registry_test

import (
	"testing"

	"github.com/eguilde/egudoc/internal/registry"
)

func TestNumberFormatting(t *testing.T) {
	tests := []struct {
		prefix   string
		num      int
		expected string
	}{
		{"INT/", 1, "INT/000001"},
		{"IEST/", 42, "IEST/000042"},
		{"CTR/", 1000, "CTR/001000"},
		{"", 999999, "999999"},
	}
	for _, tt := range tests {
		got := fmt.Sprintf("%s%06d", tt.prefix, tt.num)
		if got != tt.expected {
			t.Errorf("prefix=%q num=%d: want %q got %q", tt.prefix, tt.num, tt.expected, got)
		}
	}
}
```

*(Add `import "fmt"` to the test file.)*

```bash
go test ./internal/registry/... -v -run TestNumberFormatting
```

Expected: PASS

- [ ] **Step 12.5: Commit**

```bash
git add migrations/000005_registries.sql internal/registry/
git commit -m "feat: add registry management and atomic document number generation"
```

---

## Task 13: Document model and registratura service

**Files:**
- Create: `migrations/000006_documents.sql`
- Create: `internal/registratura/model.go`
- Create: `internal/registratura/service.go`
- Create: `internal/registratura/handler.go`

- [ ] **Step 13.1: Write 000006_documents.sql**

```sql
-- migrations/000006_documents.sql
-- +migrate Up

CREATE TYPE document_status AS ENUM (
    'INREGISTRAT',          -- just registered, awaiting routing
    'ALOCAT_COMPARTIMENT',  -- assigned to a compartiment
    'IN_LUCRU',             -- being processed by a staff member
    'FLUX_APROBARE',        -- in approval chain
    'FINALIZAT',            -- fully processed/responded
    'ARHIVAT',              -- submitted to QTSP archive
    'ANULAT'                -- cancelled
);

CREATE TYPE tip_document AS ENUM (
    'INTRARE',              -- incoming document
    'IESIRE',               -- outgoing document
    'INTERN',               -- internal memo/note
    'PETITIE',              -- citizen petition
    'CONTRACT',
    'DECIZIE',
    'HOTARARE',
    'DISPOZITIE',
    'ADRESA',               -- formal institutional address
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
    nr_inregistrare         VARCHAR(50) NOT NULL,          -- generated: "INT/000042/2025"
    registru_id             UUID NOT NULL REFERENCES registre(id),
    institution_id          UUID NOT NULL REFERENCES institutions(id),

    tip                     tip_document NOT NULL,
    status                  document_status NOT NULL DEFAULT 'INREGISTRAT',
    clasificare             clasificare_document NOT NULL DEFAULT 'PUBLIC',

    -- Parties
    emitent_id              UUID REFERENCES entitati(id),   -- sender (external)
    destinatar_id           UUID REFERENCES entitati(id),   -- recipient (external)
    emitent_intern_id       UUID REFERENCES compartimente(id),   -- internal sender
    destinatar_intern_id    UUID REFERENCES compartimente(id),   -- internal recipient

    -- Content
    obiect                  TEXT NOT NULL,                  -- subject/description
    continut                TEXT,                           -- body/summary
    cuvinte_cheie           TEXT[],                         -- keywords (Law 135/2007)
    numar_file              INTEGER,                        -- number of pages

    -- Assignment (workflow state)
    compartiment_curent_id  UUID REFERENCES compartimente(id),
    user_curent_subject     VARCHAR(255),                   -- assigned staff member (JWT sub)
    awaiting_approver_subject VARCHAR(255),                 -- who must approve

    -- Dates
    data_inregistrare       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    data_document           DATE,                           -- date on the document itself
    data_termen             DATE,                           -- response deadline
    data_finalizare         TIMESTAMPTZ,
    data_arhivare           TIMESTAMPTZ,

    -- Archiving (QTSP integration)
    termen_pastrare_ani     INTEGER NOT NULL DEFAULT 10,
    archive_document_id     VARCHAR(255),                   -- QTSP archive reference
    archive_status          VARCHAR(30) DEFAULT 'NOT_ARCHIVED',

    -- eDelivery
    delivery_message_id     VARCHAR(255),                   -- QTSP delivery reference
    delivery_status         VARCHAR(30),

    -- Workflow locking
    workflow_locked_until   TIMESTAMPTZ,

    -- References
    document_parinte_id     UUID REFERENCES documente(id),  -- reply-to
    nr_document_extern      VARCHAR(100),                   -- external doc number

    -- Metadata
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

-- Document attachments (stored in MinIO)
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

-- +migrate Down
DROP TABLE IF EXISTS atasamente;
DROP TABLE IF EXISTS documente;
DROP TYPE IF EXISTS clasificare_document;
DROP TYPE IF EXISTS tip_document;
DROP TYPE IF EXISTS document_status;
```

- [ ] **Step 13.2: Write registratura/model.go**

```go
// internal/registratura/model.go
package registratura

import (
	"time"
	"github.com/google/uuid"
)

type DocumentStatus string

const (
	StatusInregistrat       DocumentStatus = "INREGISTRAT"
	StatusAlocatCompartiment DocumentStatus = "ALOCAT_COMPARTIMENT"
	StatusInLucru           DocumentStatus = "IN_LUCRU"
	StatusFluxAprobare      DocumentStatus = "FLUX_APROBARE"
	StatusFinalizat         DocumentStatus = "FINALIZAT"
	StatusArhivat           DocumentStatus = "ARHIVAT"
	StatusAnulat            DocumentStatus = "ANULAT"
)

type TipDocument string

const (
	TipIntrare     TipDocument = "INTRARE"
	TipIesire      TipDocument = "IESIRE"
	TipIntern      TipDocument = "INTERN"
	TipPetitie     TipDocument = "PETITIE"
	TipContract    TipDocument = "CONTRACT"
	TipDecizie     TipDocument = "DECIZIE"
	TipHotarare    TipDocument = "HOTARARE"
	TipDispozitie  TipDocument = "DISPOZITIE"
	TipAdresa      TipDocument = "ADRESA"
	TipNotificare  TipDocument = "NOTIFICARE"
	TipRaport      TipDocument = "RAPORT"
	TipReferat     TipDocument = "REFERAT"
	TipAdeverinta  TipDocument = "ADEVERINTA"
	TipCertificat  TipDocument = "CERTIFICAT"
	TipAutorizatie TipDocument = "AUTORIZATIE"
	TipAviz        TipDocument = "AVIZ"
)

type Clasificare string

const (
	ClasificarePublic       Clasificare = "PUBLIC"
	ClasificareIntern       Clasificare = "INTERN"
	ClasificareConfidential Clasificare = "CONFIDENTIAL"
	ClasificareSecret       Clasificare = "SECRET"
)

type Document struct {
	ID             uuid.UUID      `json:"id"`
	NrInregistrare string         `json:"nr_inregistrare"`
	RegistruID     uuid.UUID      `json:"registru_id"`
	InstitutionID  uuid.UUID      `json:"institution_id"`

	Tip            TipDocument    `json:"tip"`
	Status         DocumentStatus `json:"status"`
	Clasificare    Clasificare    `json:"clasificare"`

	EmitentID          *uuid.UUID `json:"emitent_id,omitempty"`
	DestinatarID       *uuid.UUID `json:"destinatar_id,omitempty"`
	EmitentInternID    *uuid.UUID `json:"emitent_intern_id,omitempty"`
	DestinatarInternID *uuid.UUID `json:"destinatar_intern_id,omitempty"`

	Obiect        string   `json:"obiect"`
	Continut      string   `json:"continut,omitempty"`
	Cuvintecheie  []string `json:"cuvinte_cheie,omitempty"`
	NrFile        *int     `json:"nr_file,omitempty"`

	CompartimentCurentID    *uuid.UUID `json:"compartiment_curent_id,omitempty"`
	UserCurentSubject       string     `json:"user_curent_subject,omitempty"`
	AwaitingApproverSubject string     `json:"awaiting_approver_subject,omitempty"`

	DataInregistrare time.Time  `json:"data_inregistrare"`
	DataDocument     *time.Time `json:"data_document,omitempty"`
	DataTermen       *time.Time `json:"data_termen,omitempty"`
	DataFinalizare   *time.Time `json:"data_finalizare,omitempty"`
	DataArhivare     *time.Time `json:"data_arhivare,omitempty"`

	TermenPastrareAni  int    `json:"termen_pastrare_ani"`
	ArchiveDocumentID  string `json:"archive_document_id,omitempty"`
	ArchiveStatus      string `json:"archive_status"`
	DeliveryMessageID  string `json:"delivery_message_id,omitempty"`
	DeliveryStatus     string `json:"delivery_status,omitempty"`

	DocumentParinteID   *uuid.UUID `json:"document_parinte_id,omitempty"`
	NrDocumentExtern    string     `json:"nr_document_extern,omitempty"`
	MotivanulAre        string     `json:"motiv_anulare,omitempty"`
	RejectionCount      int        `json:"rejection_count"`

	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Populated via JOIN
	Atasamente []Atasament `json:"atasamente,omitempty"`
}

type Atasament struct {
	ID          uuid.UUID `json:"id"`
	DocumentID  uuid.UUID `json:"document_id"`
	StorageKey  string    `json:"-"` // never expose storage keys
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	Description string    `json:"description,omitempty"`
	UploadedBy  string    `json:"uploaded_by"`
	CreatedAt   time.Time `json:"created_at"`
	DownloadURL string    `json:"download_url,omitempty"` // pre-signed, populated on demand
}

type CreateDocumentDTO struct {
	RegistruID         uuid.UUID   `json:"registru_id"`
	Tip                TipDocument `json:"tip"`
	Clasificare        Clasificare `json:"clasificare"`
	EmitentID          *uuid.UUID  `json:"emitent_id"`
	DestinatarID       *uuid.UUID  `json:"destinatar_id"`
	EmitentInternID    *uuid.UUID  `json:"emitent_intern_id"`
	DestinatarInternID *uuid.UUID  `json:"destinatar_intern_id"`
	Obiect             string      `json:"obiect"`
	Continut           string      `json:"continut"`
	CuvinteChecheie    []string    `json:"cuvinte_cheie"`
	NrFile             *int        `json:"nr_file"`
	DataDocument       *time.Time  `json:"data_document"`
	DataTermen         *time.Time  `json:"data_termen"`
	NrDocumentExtern   string      `json:"nr_document_extern"`
	DocumentParinteID  *uuid.UUID  `json:"document_parinte_id"`
}

type ListDocumenteParams struct {
	InstitutionID      uuid.UUID
	CompartimentID     *uuid.UUID
	UserSubject        string
	Status             DocumentStatus
	Tip                TipDocument
	RegistruID         *uuid.UUID
	Search             string
	DataDe             *time.Time
	DataPana           *time.Time
	Page               int
	Limit              int
	SortBy             string // "data_inregistrare", "nr_inregistrare", "status"
	SortDir            string // "ASC", "DESC"
}
```

- [ ] **Step 13.3: Commit**

```bash
git add migrations/000006_documents.sql internal/registratura/model.go
git commit -m "feat: add document schema (documente + atasamente) and model"
```

---

## Task 14: Workflow engine and audit

**Files:**
- Create: `migrations/000007_workflow.sql`
- Create: `migrations/000008_approval_chains.sql`
- Create: `internal/workflow/model.go`
- Create: `internal/workflow/engine.go`
- Create: `internal/workflow/audit.go`
- Create: `internal/workflow/service.go`
- Create: `internal/workflow/handler.go`

- [ ] **Step 14.1: Write 000007_workflow.sql**

```sql
-- migrations/000007_workflow.sql
-- +migrate Up

-- Immutable audit trail of all workflow state transitions
CREATE TABLE workflow_events (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    document_id     UUID NOT NULL REFERENCES documente(id),
    institution_id  UUID NOT NULL REFERENCES institutions(id),

    action          VARCHAR(100) NOT NULL,
    -- CREATE, ASSIGN_COMPARTIMENT, ASSIGN_USER, SEND_FOR_APPROVAL,
    -- APPROVE, REJECT, FINALIZE, CANCEL, ARCHIVE, DELIVER, COMMENT

    old_status      VARCHAR(50),
    new_status      VARCHAR(50) NOT NULL,

    actor_subject   VARCHAR(255) NOT NULL,     -- JWT sub of the user taking action
    actor_ip        VARCHAR(45),

    from_compartiment_id  UUID REFERENCES compartimente(id),
    to_compartiment_id    UUID REFERENCES compartimente(id),
    assigned_user_subject VARCHAR(255),

    -- For rejection: reason text
    motiv           TEXT,
    -- Arbitrary metadata (changed fields, metadata for eDelivery, etc.)
    metadata        JSONB,

    -- Visibility: who can see this event
    visibility      VARCHAR(30) NOT NULL DEFAULT 'WORKFLOW_ONLY',
    -- WORKFLOW_ONLY: assigned user + department head + registrar + admin
    -- DEPARTMENT: all compartiment staff + above
    -- PUBLIC: all institution staff + above

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Make the event log append-only via trigger
CREATE OR REPLACE FUNCTION prevent_workflow_event_modification()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'workflow_events records are immutable';
END;
$$;

CREATE TRIGGER workflow_events_immutable
BEFORE UPDATE OR DELETE ON workflow_events
FOR EACH ROW EXECUTE FUNCTION prevent_workflow_event_modification();

CREATE INDEX idx_workflow_events_document ON workflow_events(document_id, created_at DESC);
CREATE INDEX idx_workflow_events_actor ON workflow_events(actor_subject, created_at DESC);
CREATE INDEX idx_workflow_events_institution ON workflow_events(institution_id, created_at DESC);

-- +migrate Down
DROP TRIGGER IF EXISTS workflow_events_immutable ON workflow_events;
DROP FUNCTION IF EXISTS prevent_workflow_event_modification();
DROP TABLE IF EXISTS workflow_events;
```

- [ ] **Step 14.2: Write 000008_approval_chains.sql**

```sql
-- migrations/000008_approval_chains.sql
-- +migrate Up

-- Configurable approval chains per compartiment (and optionally per document type)
CREATE TABLE lant_aprobare (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    institution_id    UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    compartiment_id   UUID REFERENCES compartimente(id),  -- NULL = institution-wide default
    tip_document      VARCHAR(100),                       -- NULL = all document types
    -- The subject of the user who must approve (or role code if role-based routing)
    approver_subject  VARCHAR(255),
    approver_role     VARCHAR(100),   -- if set, any user with this role in compartiment can approve
    ordine            INTEGER NOT NULL DEFAULT 1,         -- approval step order
    obligatoriu       BOOLEAN NOT NULL DEFAULT TRUE,
    active            BOOLEAN NOT NULL DEFAULT TRUE,
    UNIQUE(institution_id, compartiment_id, tip_document, ordine)
);

CREATE INDEX idx_lant_aprobare_compartiment ON lant_aprobare(compartiment_id, ordine) WHERE active = TRUE;

-- +migrate Down
DROP TABLE IF EXISTS lant_aprobare;
```

- [ ] **Step 14.3: Write workflow/model.go**

```go
// internal/workflow/model.go
package workflow

import (
	"time"
	"github.com/google/uuid"
)

type Action string

const (
	ActionCreate            Action = "CREATE"
	ActionAssignCompartiment Action = "ASSIGN_COMPARTIMENT"
	ActionAssignUser        Action = "ASSIGN_USER"
	ActionSendForApproval   Action = "SEND_FOR_APPROVAL"
	ActionApprove           Action = "APPROVE"
	ActionReject            Action = "REJECT"
	ActionFinalize          Action = "FINALIZE"
	ActionCancel            Action = "CANCEL"
	ActionAddComment        Action = "ADD_COMMENT"
)

type Visibility string

const (
	VisibilityWorkflowOnly Visibility = "WORKFLOW_ONLY"
	VisibilityDepartment   Visibility = "DEPARTMENT"
	VisibilityPublic       Visibility = "PUBLIC"
)

type WorkflowEvent struct {
	ID             uuid.UUID  `json:"id"`
	DocumentID     uuid.UUID  `json:"document_id"`
	InstitutionID  uuid.UUID  `json:"institution_id"`

	Action         Action     `json:"action"`
	OldStatus      string     `json:"old_status,omitempty"`
	NewStatus      string     `json:"new_status"`

	ActorSubject             string     `json:"actor_subject"`
	ActorIP                  string     `json:"actor_ip,omitempty"`
	FromCompartimentID       *uuid.UUID `json:"from_compartiment_id,omitempty"`
	ToCompartimentID         *uuid.UUID `json:"to_compartiment_id,omitempty"`
	AssignedUserSubject      string     `json:"assigned_user_subject,omitempty"`
	Motiv                    string     `json:"motiv,omitempty"`
	Metadata                 map[string]any `json:"metadata,omitempty"`
	Visibility               Visibility `json:"visibility"`
	CreatedAt                time.Time  `json:"created_at"`
}

// ActionRequest is the payload for any workflow action.
type ActionRequest struct {
	Action              Action     `json:"action"`
	CompartimentID      *uuid.UUID `json:"compartiment_id,omitempty"`
	AssigneeSubject     string     `json:"assignee_subject,omitempty"`
	Motiv               string     `json:"motiv,omitempty"`
}

// ValidTransitions defines the allowed state machine transitions.
// Key: (currentStatus, action) → newStatus
var ValidTransitions = map[string]map[Action]string{
	"INREGISTRAT": {
		ActionAssignCompartiment: "ALOCAT_COMPARTIMENT",
		ActionCancel:             "ANULAT",
	},
	"ALOCAT_COMPARTIMENT": {
		ActionAssignUser: "IN_LUCRU",
		ActionCancel:     "ANULAT",
	},
	"IN_LUCRU": {
		ActionSendForApproval: "FLUX_APROBARE",
		ActionFinalize:        "FINALIZAT",   // direct finalization without approval
		ActionCancel:          "ANULAT",
	},
	"FLUX_APROBARE": {
		ActionApprove: "FINALIZAT",
		ActionReject:  "IN_LUCRU",
		ActionCancel:  "ANULAT",
	},
	"FINALIZAT": {}, // terminal state — archiving is a side effect, not a status transition
	"ANULAT":    {}, // terminal state
}
```

- [ ] **Step 14.4: Write workflow/engine.go**

```go
// internal/workflow/engine.go
package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/eguilde/egudoc/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Engine executes validated workflow transitions on documents.
// All transitions are wrapped in a DB transaction to ensure atomicity.
type Engine struct {
	db    *pgxpool.Pool
	audit *AuditService
}

func NewEngine(db *pgxpool.Pool) *Engine {
	return &Engine{db: db, audit: NewAuditService(db)}
}

// Advance processes a workflow action on a document.
// It validates the transition, updates the document, and logs the event.
func (e *Engine) Advance(ctx context.Context, documentID uuid.UUID, req ActionRequest, claims *auth.Claims, ip string) (*WorkflowEvent, error) {
	tx, err := e.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock the document row
	var currentStatus string
	var institutionID uuid.UUID
	var lockedUntil *time.Time
	err = tx.QueryRow(ctx, `
		SELECT status, institution_id, workflow_locked_until
		FROM documente WHERE id = $1
		FOR UPDATE
	`, documentID).Scan(&currentStatus, &institutionID, &lockedUntil)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("document not found")
	}
	if err != nil {
		return nil, fmt.Errorf("lock document: %w", err)
	}

	// Check workflow lock
	if lockedUntil != nil && time.Now().Before(*lockedUntil) {
		return nil, fmt.Errorf("document is locked until %s", lockedUntil.Format(time.RFC3339))
	}

	// Validate transition
	transitions, ok := ValidTransitions[currentStatus]
	if !ok {
		return nil, fmt.Errorf("unknown document status: %s", currentStatus)
	}
	newStatus, ok := transitions[req.Action]
	if !ok {
		return nil, fmt.Errorf("action %q is not allowed when document status is %q", req.Action, currentStatus)
	}

	// Build the UPDATE for the document
	update := map[string]any{
		"status":     newStatus,
		"updated_at": time.Now(),
	}

	switch req.Action {
	case ActionAssignCompartiment:
		if req.CompartimentID == nil {
			return nil, fmt.Errorf("compartiment_id required for ASSIGN_COMPARTIMENT")
		}
		update["compartiment_curent_id"] = req.CompartimentID
		update["user_curent_subject"] = nil
		update["awaiting_approver_subject"] = nil

	case ActionAssignUser:
		if req.AssigneeSubject == "" {
			return nil, fmt.Errorf("assignee_subject required for ASSIGN_USER")
		}
		update["user_curent_subject"] = req.AssigneeSubject

	case ActionSendForApproval:
		if req.AssigneeSubject == "" {
			return nil, fmt.Errorf("assignee_subject (approver) required for SEND_FOR_APPROVAL")
		}
		update["awaiting_approver_subject"] = req.AssigneeSubject
		update["user_curent_subject"] = nil

	case ActionApprove:
		update["awaiting_approver_subject"] = nil
		update["user_curent_subject"] = nil
		update["workflow_locked_until"] = nil
		update["data_finalizare"] = time.Now()

	case ActionReject:
		update["awaiting_approver_subject"] = nil
		// Return to the user who sent for approval — stored in event history
		update["rejection_count"] = "(rejection_count + 1)"

	case ActionFinalize:
		update["data_finalizare"] = time.Now()
		update["workflow_locked_until"] = nil

	case ActionCancel:
		if req.Motiv == "" {
			return nil, fmt.Errorf("motiv required for CANCEL")
		}
		update["motiv_anulare"] = req.Motiv
		update["workflow_locked_until"] = nil
	}

	// Apply the update
	_, err = tx.Exec(ctx, `
		UPDATE documente SET
			status = $1,
			compartiment_curent_id = COALESCE($2, compartiment_curent_id),
			user_curent_subject = $3,
			awaiting_approver_subject = $4,
			workflow_locked_until = $5,
			rejection_count = CASE WHEN $6 THEN rejection_count + 1 ELSE rejection_count END,
			data_finalizare = $7,
			motiv_anulare = $8,
			updated_at = NOW()
		WHERE id = $9
	`,
		newStatus,
		req.CompartimentID,
		nullableStr(req.AssigneeSubject),
		nullableStr(update["awaiting_approver_subject"]),
		update["workflow_locked_until"],
		req.Action == ActionReject,
		update["data_finalizare"],
		update["motiv_anulare"],
		documentID,
	)
	if err != nil {
		return nil, fmt.Errorf("update document: %w", err)
	}

	// Log the event
	event := WorkflowEvent{
		DocumentID:    documentID,
		InstitutionID: institutionID,
		Action:        req.Action,
		OldStatus:     currentStatus,
		NewStatus:     newStatus,
		ActorSubject:  claims.Subject,
		ActorIP:       ip,
		Motiv:         req.Motiv,
		Visibility:    VisibilityWorkflowOnly,
	}
	if req.CompartimentID != nil {
		event.ToCompartimentID = req.CompartimentID
	}

	if err := e.audit.LogEventTx(ctx, tx, event); err != nil {
		return nil, fmt.Errorf("log event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transition: %w", err)
	}

	return &event, nil
}

func nullableStr(v any) *string {
	if v == nil {
		return nil
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return nil
	}
	return &s
}
```

- [ ] **Step 14.5: Write workflow/audit.go**

```go
// internal/workflow/audit.go
package workflow

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditService struct {
	db *pgxpool.Pool
}

func NewAuditService(db *pgxpool.Pool) *AuditService {
	return &AuditService{db: db}
}

// LogEventTx writes an immutable workflow event within an existing transaction.
func (s *AuditService) LogEventTx(ctx context.Context, tx pgx.Tx, event WorkflowEvent) error {
	metaJSON, _ := json.Marshal(event.Metadata)

	_, err := tx.Exec(ctx, `
		INSERT INTO workflow_events
		  (document_id, institution_id, action, old_status, new_status,
		   actor_subject, actor_ip,
		   from_compartiment_id, to_compartiment_id, assigned_user_subject,
		   motiv, metadata, visibility)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`,
		event.DocumentID, event.InstitutionID, event.Action,
		event.OldStatus, event.NewStatus,
		event.ActorSubject, event.ActorIP,
		event.FromCompartimentID, event.ToCompartimentID, event.AssignedUserSubject,
		event.Motiv, metaJSON, event.Visibility,
	)
	if err != nil {
		return fmt.Errorf("insert workflow event: %w", err)
	}
	return nil
}

// GetAuditTrail returns the workflow history for a document.
// Filters events based on the requesting user's role (RBAC-based visibility).
func (s *AuditService) GetAuditTrail(ctx context.Context, documentID, institutionID string, isAdmin bool) ([]WorkflowEvent, error) {
	query := `
		SELECT id, document_id, institution_id, action, old_status, new_status,
		       actor_subject, actor_ip,
		       from_compartiment_id, to_compartiment_id, assigned_user_subject,
		       motiv, metadata, visibility, created_at
		FROM workflow_events
		WHERE document_id = $1
		ORDER BY created_at ASC
	`
	rows, err := s.db.Query(ctx, query, documentID)
	if err != nil {
		return nil, fmt.Errorf("query audit trail: %w", err)
	}
	defer rows.Close()

	var events []WorkflowEvent
	for rows.Next() {
		var e WorkflowEvent
		var metaJSON []byte
		if err := rows.Scan(
			&e.ID, &e.DocumentID, &e.InstitutionID, &e.Action, &e.OldStatus, &e.NewStatus,
			&e.ActorSubject, &e.ActorIP,
			&e.FromCompartimentID, &e.ToCompartimentID, &e.AssignedUserSubject,
			&e.Motiv, &metaJSON, &e.Visibility, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		if len(metaJSON) > 0 {
			json.Unmarshal(metaJSON, &e.Metadata)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
```

- [ ] **Step 14.6: Write workflow engine test**

```go
// internal/workflow/engine_test.go
package workflow_test

import (
	"testing"

	"github.com/eguilde/egudoc/internal/workflow"
)

func TestValidTransitions(t *testing.T) {
	tests := []struct {
		fromStatus string
		action     workflow.Action
		wantStatus string
		wantErr    bool
	}{
		{"INREGISTRAT", workflow.ActionAssignCompartiment, "ALOCAT_COMPARTIMENT", false},
		{"ALOCAT_COMPARTIMENT", workflow.ActionAssignUser, "IN_LUCRU", false},
		{"IN_LUCRU", workflow.ActionSendForApproval, "FLUX_APROBARE", false},
		{"FLUX_APROBARE", workflow.ActionApprove, "FINALIZAT", false},
		{"FLUX_APROBARE", workflow.ActionReject, "IN_LUCRU", false},
		{"FINALIZAT", workflow.ActionApprove, "", true},  // no transitions from FINALIZAT
		{"IN_LUCRU", workflow.ActionApprove, "", true},   // can't approve from IN_LUCRU
	}

	for _, tt := range tests {
		transitions, ok := workflow.ValidTransitions[tt.fromStatus]
		if !ok && !tt.wantErr {
			t.Errorf("status %q has no transitions", tt.fromStatus)
			continue
		}
		newStatus, ok := transitions[tt.action]
		if !ok {
			if !tt.wantErr {
				t.Errorf("from=%q action=%q: expected transition but got none", tt.fromStatus, tt.action)
			}
			continue
		}
		if tt.wantErr {
			t.Errorf("from=%q action=%q: expected error but got %q", tt.fromStatus, tt.action, newStatus)
		}
		if newStatus != tt.wantStatus {
			t.Errorf("from=%q action=%q: want %q got %q", tt.fromStatus, tt.action, tt.wantStatus, newStatus)
		}
	}
}
```

```bash
go test ./internal/workflow/... -v -run TestValidTransitions
```

Expected: PASS

- [ ] **Step 14.7: Commit**

```bash
git add migrations/000007_workflow.sql migrations/000008_approval_chains.sql internal/workflow/
git commit -m "feat: add workflow engine with state machine, immutable audit trail, approval chains"
git push origin master
```

---

## Sub-plan B Completion Checklist

- [ ] `entitati` table created and service compiles
- [ ] `registre` table with atomic number generator
- [ ] `documente` + `atasamente` tables
- [ ] `workflow_events` table with immutable trigger
- [ ] `lant_aprobare` table
- [ ] ValidTransitions test passes
- [ ] Number formatting test passes
- [ ] `go build ./...` compiles cleanly

---

*Next: Sub-plan C — Frontend (Angular 21 + PrimeNG 21)*
