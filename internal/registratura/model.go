// internal/registratura/model.go
package registratura

import (
	"time"

	"github.com/google/uuid"
)

type DocumentStatus string

const (
	StatusInregistrat        DocumentStatus = "INREGISTRAT"
	StatusAlocatCompartiment DocumentStatus = "ALOCAT_COMPARTIMENT"
	StatusInLucru            DocumentStatus = "IN_LUCRU"
	StatusFluxAprobare       DocumentStatus = "FLUX_APROBARE"
	StatusFinalizat          DocumentStatus = "FINALIZAT"
	StatusArhivat            DocumentStatus = "ARHIVAT"
	StatusAnulat             DocumentStatus = "ANULAT"
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
	ID             uuid.UUID `json:"id"`
	NrInregistrare string    `json:"nr_inregistrare"`
	RegistruID     uuid.UUID `json:"registru_id"`
	InstitutionID  uuid.UUID `json:"institution_id"`

	Tip         TipDocument    `json:"tip"`
	Status      DocumentStatus `json:"status"`
	Clasificare Clasificare    `json:"clasificare"`

	EmitentID          *uuid.UUID `json:"emitent_id,omitempty"`
	DestinatarID       *uuid.UUID `json:"destinatar_id,omitempty"`
	EmitentInternID    *uuid.UUID `json:"emitent_intern_id,omitempty"`
	DestinatarInternID *uuid.UUID `json:"destinatar_intern_id,omitempty"`

	Obiect          string   `json:"obiect"`
	Continut        string   `json:"continut,omitempty"`
	CuvinteChecheie []string `json:"cuvinte_cheie,omitempty"`
	NrFile          *int     `json:"nr_file,omitempty"`

	CompartimentCurentID    *uuid.UUID `json:"compartiment_curent_id,omitempty"`
	UserCurentSubject       string     `json:"user_curent_subject,omitempty"`
	AwaitingApproverSubject string     `json:"awaiting_approver_subject,omitempty"`

	DataInregistrare time.Time  `json:"data_inregistrare"`
	DataDocument     *time.Time `json:"data_document,omitempty"`
	DataTermen       *time.Time `json:"data_termen,omitempty"`
	DataFinalizare   *time.Time `json:"data_finalizare,omitempty"`
	DataArhivare     *time.Time `json:"data_arhivare,omitempty"`

	TermenPastrareAni int    `json:"termen_pastrare_ani"`
	ArchiveDocumentID string `json:"archive_document_id,omitempty"`
	ArchiveStatus     string `json:"archive_status"`
	DeliveryMessageID string `json:"delivery_message_id,omitempty"`
	DeliveryStatus    string `json:"delivery_status,omitempty"`

	DocumentParinteID *uuid.UUID `json:"document_parinte_id,omitempty"`
	NrDocumentExtern  string     `json:"nr_document_extern,omitempty"`
	MotivAnulare      string     `json:"motiv_anulare,omitempty"`
	RejectionCount    int        `json:"rejection_count"`

	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Populated via JOIN
	Atasamente []Atasament `json:"atasamente,omitempty"`
}

type Atasament struct {
	ID          uuid.UUID `json:"id"`
	DocumentID  uuid.UUID `json:"document_id"`
	StorageKey  string    `json:"-"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	Description string    `json:"description,omitempty"`
	UploadedBy  string    `json:"uploaded_by"`
	CreatedAt   time.Time `json:"created_at"`
	DownloadURL string    `json:"download_url,omitempty"`
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
	InstitutionID  uuid.UUID
	CompartimentID *uuid.UUID
	UserSubject    string
	Status         DocumentStatus
	Tip            TipDocument
	RegistruID     *uuid.UUID
	Search         string
	DataDe         *time.Time
	DataPana       *time.Time
	Page           int
	Limit          int
	SortBy         string
	SortDir        string
}
