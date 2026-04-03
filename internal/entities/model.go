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
	ID         uuid.UUID   `json:"id"`
	Tip        TipEntitate `json:"tip"`
	Denumire   string      `json:"denumire"`
	Adresa     string      `json:"adresa,omitempty"`
	Localitate string      `json:"localitate,omitempty"`
	Judet      string      `json:"judet,omitempty"`
	Telefon    string      `json:"telefon,omitempty"`
	Email      string      `json:"email,omitempty"`

	// Persoana fizica
	CNP          string     `json:"cnp,omitempty"`
	Prenume      string     `json:"prenume,omitempty"`
	DataNasterii *time.Time `json:"data_nasterii,omitempty"`
	LocNasterii  string     `json:"loc_nasterii,omitempty"`

	// Persoana juridica
	CUI               string `json:"cui,omitempty"`
	NrRegCom          string `json:"nr_reg_com,omitempty"`
	ReprezentantLegal string `json:"reprezentant_legal,omitempty"`
	FormaJuridica     string `json:"forma_juridica,omitempty"`

	// Institutie publica
	CodSiruta             string `json:"cod_siruta,omitempty"`
	NivelInstitutie       string `json:"nivel_institutie,omitempty"`
	TipInstitutie         string `json:"tip_institutie,omitempty"`
	Website               string `json:"website,omitempty"`
	DeliveryParticipantID string `json:"delivery_participant_id,omitempty"`

	InstitutionID *uuid.UUID `json:"institution_id,omitempty"`
	CreatedBy     string     `json:"created_by"`
	Active        bool       `json:"active"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type CreateEntitateDTO struct {
	Tip        TipEntitate `json:"tip"`
	Denumire   string      `json:"denumire"`
	Adresa     string      `json:"adresa"`
	Localitate string      `json:"localitate"`
	Judet      string      `json:"judet"`
	Telefon    string      `json:"telefon"`
	Email      string      `json:"email"`
	// Persoana fizica
	CNP     string `json:"cnp"`
	Prenume string `json:"prenume"`
	// Persoana juridica
	CUI      string `json:"cui"`
	NrRegCom string `json:"nr_reg_com"`
	// Institutie
	CodSiruta     string `json:"cod_siruta"`
	TipInstitutie string `json:"tip_institutie"`
}

type ListEntitatiParams struct {
	Search        string
	Tip           TipEntitate
	InstitutionID *uuid.UUID
	Page          int
	Limit         int
}
