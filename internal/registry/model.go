// internal/registry/model.go
package registry

import (
	"time"

	"github.com/google/uuid"
)

type TipRegistru string

const (
	TipIntrari    TipRegistru = "INTRARI"
	TipIesiri     TipRegistru = "IESIRI"
	TipIntern     TipRegistru = "INTERN"
	TipPetitii    TipRegistru = "PETITII"
	TipContracte  TipRegistru = "CONTRACTE"
	TipDecizii    TipRegistru = "DECIZII"
	TipHotarari   TipRegistru = "HOTARARI"
	TipDispozitii TipRegistru = "DISPOZITII"
	TipGeneral    TipRegistru = "GENERAL"
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
