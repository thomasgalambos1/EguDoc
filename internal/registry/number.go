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
// Uses Serializable isolation to prevent race conditions.
// Returns the formatted document number: "{prefix}{NNNNNN}" (zero-padded to 6 digits).
func GenerateNextNumber(ctx context.Context, pool *pgxpool.Pool, registryID uuid.UUID) (string, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

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

	return fmt.Sprintf("%s%06d", prefix, assigned), nil
}
