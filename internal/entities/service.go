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
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit < 1 || params.Limit > 100 {
		params.Limit = 20
	}
	offset := (params.Page - 1) * params.Limit

	conditions := []string{"active = TRUE"}
	args := []any{}
	i := 1

	if params.Tip != "" {
		conditions = append(conditions, fmt.Sprintf("tip = $%d", i))
		args = append(args, params.Tip)
		i++
	}
	if params.InstitutionID != nil {
		conditions = append(conditions, fmt.Sprintf("institution_id = $%d", i))
		args = append(args, params.InstitutionID)
		i++
	}
	if params.Search != "" {
		conditions = append(conditions, fmt.Sprintf(
			"(denumire ILIKE $%d OR cui ILIKE $%d OR email ILIKE $%d)", i, i+1, i+2,
		))
		q := "%" + params.Search + "%"
		args = append(args, q, q, q)
		i += 3
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	var total int
	if err := s.db.QueryRow(ctx, "SELECT COUNT(*) FROM entitati "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count entitati: %w", err)
	}

	args = append(args, params.Limit, offset)
	query := fmt.Sprintf(`
		SELECT id, tip, denumire, adresa, localitate, judet, telefon, email,
		       cnp, prenume, cui, nr_reg_com, cod_siruta, tip_institutie, delivery_participant_id,
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
			&e.CNP, &e.Prenume, &e.CUI, &e.NrRegCom, &e.CodSiruta, &e.TipInstitutie, &e.DeliveryParticipantID,
			&e.InstitutionID, &e.CreatedBy, &e.Active, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan entitate: %w", err)
		}
		result = append(result, &e)
	}
	return result, total, rows.Err()
}
