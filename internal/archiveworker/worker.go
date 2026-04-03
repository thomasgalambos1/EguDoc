package archiveworker

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type Worker struct {
	db           *pgxpool.Pool
	pipeline     *Pipeline
	log          *zap.Logger
	interval     time.Duration
	archiveDelay time.Duration
}

func NewWorker(db *pgxpool.Pool, pipeline *Pipeline, log *zap.Logger) *Worker {
	return &Worker{
		db:           db,
		pipeline:     pipeline,
		log:          log,
		interval:     6 * time.Hour,
		archiveDelay: 30 * 24 * time.Hour,
	}
}

func (w *Worker) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		w.runCycle(ctx)

		for {
			select {
			case <-ctx.Done():
				w.log.Info("archive worker stopping")
				return
			case <-ticker.C:
				w.runCycle(ctx)
			}
		}
	}()
}

func (w *Worker) runCycle(ctx context.Context) {
	w.log.Info("archive worker: scanning for documents to archive")

	candidates, err := w.findCandidates(ctx)
	if err != nil {
		w.log.Error("archive worker: find candidates failed", zap.Error(err))
		return
	}

	w.log.Info("archive worker: found candidates", zap.Int("count", len(candidates)))

	for _, id := range candidates {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := w.pipeline.ArchiveDocument(ctx, id); err != nil {
			w.log.Error("archive worker: pipeline failed",
				zap.String("document_id", id.String()),
				zap.Error(err),
			)
		} else {
			w.log.Info("archive worker: document archived",
				zap.String("document_id", id.String()),
			)
		}
	}
}

func (w *Worker) findCandidates(ctx context.Context) ([]uuid.UUID, error) {
	cutoff := time.Now().Add(-w.archiveDelay)
	rows, err := w.db.Query(ctx, `
		SELECT id FROM documente
		WHERE status = 'FINALIZAT'
		  AND archive_status = 'NOT_ARCHIVED'
		  AND data_finalizare IS NOT NULL
		  AND data_finalizare < $1
		ORDER BY data_finalizare ASC
		LIMIT 50
	`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (w *Worker) TriggerImmediate(ctx context.Context, documentID uuid.UUID) {
	go func() {
		if err := w.pipeline.ArchiveDocument(ctx, documentID); err != nil {
			w.log.Error("immediate archive failed",
				zap.String("document_id", documentID.String()),
				zap.Error(err),
			)
		}
	}()
}
