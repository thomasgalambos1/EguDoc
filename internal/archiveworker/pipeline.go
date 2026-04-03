package archiveworker

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eguilde/egudoc/internal/eark"
	"github.com/eguilde/egudoc/internal/pdf"
	qtsparchive "github.com/eguilde/egudoc/internal/qtsp/archive"
	"github.com/eguilde/egudoc/internal/storage"
)

type Pipeline struct {
	db         *pgxpool.Pool
	store      *storage.Client
	gotenberg  *pdf.Gotenberg
	qtspClient *qtsparchive.Client
}

func NewPipeline(db *pgxpool.Pool, store *storage.Client, gotenberg *pdf.Gotenberg, qtsp *qtsparchive.Client) *Pipeline {
	return &Pipeline{db: db, store: store, gotenberg: gotenberg, qtspClient: qtsp}
}

// ArchiveDocument runs the full archive pipeline for one document.
func (p *Pipeline) ArchiveDocument(ctx context.Context, documentID uuid.UUID) error {
	if _, err := p.db.Exec(ctx, `UPDATE documente SET archive_status = 'PENDING', updated_at = NOW() WHERE id = $1`, documentID); err != nil {
		return fmt.Errorf("mark pending: %w", err)
	}

	doc, meta, err := p.fetchDocumentAndMeta(ctx, documentID)
	if err != nil {
		p.markFailed(ctx, documentID)
		return err
	}

	var docContent []byte
	var docFilename string

	if doc.StorageKey != "" {
		reader, _, err := p.store.GetDocument(ctx, doc.StorageKey)
		if err != nil {
			p.markFailed(ctx, documentID)
			return fmt.Errorf("fetch from storage: %w", err)
		}
		defer reader.Close()

		docContent, err = p.gotenberg.ConvertToPDFA(ctx, doc.Filename, reader)
		if err != nil {
			p.markFailed(ctx, documentID)
			return fmt.Errorf("pdf/a conversion: %w", err)
		}
		docFilename = replaceExt(doc.Filename, ".pdf")
	} else {
		html := buildDocumentHTML(doc)
		docContent, err = p.gotenberg.ConvertHTMLToPDFA(ctx, html, doc.NrInregistrare)
		if err != nil {
			p.markFailed(ctx, documentID)
			return fmt.Errorf("html to pdfa: %w", err)
		}
		docFilename = "document.pdf"
	}

	sipData, _, err := eark.BuildSIPAndStream(ctx, eark.SIPInput{
		PackageID:        documentID.String(),
		Label:            doc.NrInregistrare + " - " + doc.Obiect,
		DocumentContent:  docContent,
		DocumentFilename: docFilename,
		Metadata:         meta,
		Events:           doc.WorkflowEvents,
	})
	if err != nil {
		p.markFailed(ctx, documentID)
		return fmt.Errorf("build SIP: %w", err)
	}

	title := fmt.Sprintf("%s - %s", doc.NrInregistrare, doc.Obiect)
	result, err := p.qtspClient.Ingest(ctx, title, doc.InstitutionCUI, doc.TermenPastrareAni, sipData, documentID.String()+"-SIP.zip", "application/zip")
	if err != nil {
		p.markFailed(ctx, documentID)
		return fmt.Errorf("QTSP ingest: %w", err)
	}

	_, err = p.db.Exec(ctx, `
		UPDATE documente
		SET archive_document_id = $1, archive_status = 'ARCHIVED', data_arhivare = $2, updated_at = NOW()
		WHERE id = $3
	`, result.ID, time.Now(), documentID)
	if err != nil {
		return fmt.Errorf("update archive reference: %w", err)
	}
	return nil
}

func (p *Pipeline) markFailed(ctx context.Context, documentID uuid.UUID) {
	p.db.Exec(ctx, `UPDATE documente SET archive_status = 'FAILED', updated_at = NOW() WHERE id = $1`, documentID)
}

type docRecord struct {
	NrInregistrare    string
	Obiect            string
	StorageKey        string
	Filename          string
	InstitutionCUI    string
	TermenPastrareAni int
	WorkflowEvents    []eark.WorkflowEventForPREMIS
}

func (p *Pipeline) fetchDocumentAndMeta(ctx context.Context, documentID uuid.UUID) (*docRecord, eark.DocumentMetadata, error) {
	var doc docRecord
	var meta eark.DocumentMetadata

	// atasamente has no is_primary column - pick first attachment by created_at
	err := p.db.QueryRow(ctx, `
		SELECT d.nr_inregistrare, d.obiect, d.termen_pastrare_ani,
		       COALESCE(a.storage_key, '') as storage_key,
		       COALESCE(a.filename, '') as filename,
		       i.cui, i.denumire,
		       COALESCE(e.denumire, '') as emitent_denumire
		FROM documente d
		JOIN institutions i ON i.id = d.institution_id
		LEFT JOIN LATERAL (
			SELECT storage_key, filename FROM atasamente
			WHERE document_id = d.id
			ORDER BY created_at ASC
			LIMIT 1
		) a ON TRUE
		LEFT JOIN entitati e ON e.id = d.emitent_id
		WHERE d.id = $1
	`, documentID).Scan(
		&doc.NrInregistrare, &doc.Obiect, &doc.TermenPastrareAni,
		&doc.StorageKey, &doc.Filename,
		&doc.InstitutionCUI, &meta.InstitutionDenumire,
		&meta.EmitentDenumire,
	)
	if err != nil {
		return nil, meta, fmt.Errorf("fetch document: %w", err)
	}

	meta.NrInregistrare = doc.NrInregistrare
	meta.InstitutionCUI = doc.InstitutionCUI
	meta.Obiect = doc.Obiect
	meta.TermenPastrareAni = doc.TermenPastrareAni

	rows, err := p.db.Query(ctx, `
		SELECT action, actor_subject, COALESCE(motiv, action), created_at
		FROM workflow_events
		WHERE document_id = $1
		ORDER BY created_at ASC
	`, documentID)
	if err != nil {
		return nil, meta, fmt.Errorf("fetch events: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var we eark.WorkflowEventForPREMIS
		if err := rows.Scan(&we.Action, &we.ActorSubject, &we.Detail, &we.OccurredAt); err != nil {
			return nil, meta, fmt.Errorf("scan event: %w", err)
		}
		doc.WorkflowEvents = append(doc.WorkflowEvents, we)
	}

	return &doc, meta, rows.Err()
}

func buildDocumentHTML(doc *docRecord) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="ro"><head><meta charset="UTF-8"><title>%s</title></head>
<body><h1>%s</h1><p><strong>Nr. Înregistrare:</strong> %s</p><p><strong>Obiect:</strong> %s</p></body></html>`,
		doc.NrInregistrare, doc.NrInregistrare, doc.NrInregistrare, doc.Obiect)
}

func replaceExt(filename, newExt string) string {
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			return filename[:i] + newExt
		}
	}
	return filename + newExt
}

