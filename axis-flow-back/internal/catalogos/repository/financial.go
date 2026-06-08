package repository

import (
	"context"
	"errors"
	"fmt"

	"axis-flow-back/internal/catalogos/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BankRepo handles catalogos.catalog_banks.
type BankRepo struct{ pool *pgxpool.Pool }

func NewBankRepo(pool *pgxpool.Pool) *BankRepo { return &BankRepo{pool: pool} }

func (r *BankRepo) List(ctx context.Context) ([]domain.Bank, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, code, name, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_banks WHERE deleted_at IS NULL ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("BankRepo.List: %w", err)
	}
	defer rows.Close()

	var out []domain.Bank
	for rows.Next() {
		var b domain.Bank
		if err := rows.Scan(&b.ID, &b.Code, &b.Name, &b.DeletedAt, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("BankRepo.List scan: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (r *BankRepo) FindByID(ctx context.Context, id int64) (*domain.Bank, error) {
	var b domain.Bank
	err := r.pool.QueryRow(ctx,
		`SELECT id, code, name, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_banks WHERE id=$1 AND deleted_at IS NULL`, id).
		Scan(&b.ID, &b.Code, &b.Name, &b.DeletedAt, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("BankRepo.FindByID: %w", err)
	}
	return &b, nil
}

func (r *BankRepo) Create(ctx context.Context, b *domain.Bank) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO catalogos.catalog_banks (code, name) VALUES ($1,$2) RETURNING id, created_at, updated_at`,
		b.Code, b.Name).Scan(&b.ID, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("BankRepo.Create: %w", err)
	}
	return nil
}

func (r *BankRepo) Update(ctx context.Context, b *domain.Bank) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_banks SET code=$1, name=$2 WHERE id=$3 AND deleted_at IS NULL`,
		b.Code, b.Name, b.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("BankRepo.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *BankRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_banks SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		if isFKViolation(err) {
			return domain.ErrHasDependents
		}
		return fmt.Errorf("BankRepo.Delete: %w", err)
	}
	return nil
}

// TaxRegimeRepo handles catalogos.catalog_tax_regimes.
type TaxRegimeRepo struct{ pool *pgxpool.Pool }

func NewTaxRegimeRepo(pool *pgxpool.Pool) *TaxRegimeRepo { return &TaxRegimeRepo{pool: pool} }

func (r *TaxRegimeRepo) List(ctx context.Context) ([]domain.TaxRegime, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, code, name, persona_fisica, persona_moral, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_tax_regimes WHERE deleted_at IS NULL ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("TaxRegimeRepo.List: %w", err)
	}
	defer rows.Close()

	var out []domain.TaxRegime
	for rows.Next() {
		var t domain.TaxRegime
		if err := rows.Scan(&t.ID, &t.Code, &t.Name, &t.PersonaFisica, &t.PersonaMoral,
			&t.DeletedAt, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("TaxRegimeRepo.List scan: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *TaxRegimeRepo) FindByID(ctx context.Context, id int64) (*domain.TaxRegime, error) {
	var t domain.TaxRegime
	err := r.pool.QueryRow(ctx,
		`SELECT id, code, name, persona_fisica, persona_moral, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_tax_regimes WHERE id=$1 AND deleted_at IS NULL`, id).
		Scan(&t.ID, &t.Code, &t.Name, &t.PersonaFisica, &t.PersonaMoral, &t.DeletedAt, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("TaxRegimeRepo.FindByID: %w", err)
	}
	return &t, nil
}

func (r *TaxRegimeRepo) Create(ctx context.Context, t *domain.TaxRegime) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO catalogos.catalog_tax_regimes (code, name, persona_fisica, persona_moral)
		 VALUES ($1,$2,$3,$4) RETURNING id, created_at, updated_at`,
		t.Code, t.Name, t.PersonaFisica, t.PersonaMoral).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("TaxRegimeRepo.Create: %w", err)
	}
	return nil
}

func (r *TaxRegimeRepo) Update(ctx context.Context, t *domain.TaxRegime) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_tax_regimes SET code=$1, name=$2, persona_fisica=$3, persona_moral=$4
		 WHERE id=$5 AND deleted_at IS NULL`,
		t.Code, t.Name, t.PersonaFisica, t.PersonaMoral, t.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("TaxRegimeRepo.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *TaxRegimeRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_tax_regimes SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		if isFKViolation(err) {
			return domain.ErrHasDependents
		}
		return fmt.Errorf("TaxRegimeRepo.Delete: %w", err)
	}
	return nil
}

// PaymentFormRepo handles catalogos.catalog_payment_forms.
type PaymentFormRepo struct{ pool *pgxpool.Pool }

func NewPaymentFormRepo(pool *pgxpool.Pool) *PaymentFormRepo { return &PaymentFormRepo{pool: pool} }

func (r *PaymentFormRepo) List(ctx context.Context) ([]domain.PaymentForm, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, code, name, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_payment_forms WHERE deleted_at IS NULL ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("PaymentFormRepo.List: %w", err)
	}
	defer rows.Close()

	var out []domain.PaymentForm
	for rows.Next() {
		var p domain.PaymentForm
		if err := rows.Scan(&p.ID, &p.Code, &p.Name, &p.DeletedAt, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("PaymentFormRepo.List scan: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *PaymentFormRepo) FindByID(ctx context.Context, id int64) (*domain.PaymentForm, error) {
	var p domain.PaymentForm
	err := r.pool.QueryRow(ctx,
		`SELECT id, code, name, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_payment_forms WHERE id=$1 AND deleted_at IS NULL`, id).
		Scan(&p.ID, &p.Code, &p.Name, &p.DeletedAt, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("PaymentFormRepo.FindByID: %w", err)
	}
	return &p, nil
}

func (r *PaymentFormRepo) Create(ctx context.Context, p *domain.PaymentForm) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO catalogos.catalog_payment_forms (code, name) VALUES ($1,$2) RETURNING id, created_at, updated_at`,
		p.Code, p.Name).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("PaymentFormRepo.Create: %w", err)
	}
	return nil
}

func (r *PaymentFormRepo) Update(ctx context.Context, p *domain.PaymentForm) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_payment_forms SET code=$1, name=$2 WHERE id=$3 AND deleted_at IS NULL`,
		p.Code, p.Name, p.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("PaymentFormRepo.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *PaymentFormRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_payment_forms SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		if isFKViolation(err) {
			return domain.ErrHasDependents
		}
		return fmt.Errorf("PaymentFormRepo.Delete: %w", err)
	}
	return nil
}

// PaymentConditionRepo handles catalogos.catalog_payment_conditions.
type PaymentConditionRepo struct{ pool *pgxpool.Pool }

func NewPaymentConditionRepo(pool *pgxpool.Pool) *PaymentConditionRepo {
	return &PaymentConditionRepo{pool: pool}
}

func (r *PaymentConditionRepo) List(ctx context.Context) ([]domain.PaymentCondition, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, code, name, days, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_payment_conditions WHERE deleted_at IS NULL ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("PaymentConditionRepo.List: %w", err)
	}
	defer rows.Close()

	var out []domain.PaymentCondition
	for rows.Next() {
		var p domain.PaymentCondition
		if err := rows.Scan(&p.ID, &p.Code, &p.Name, &p.Days, &p.DeletedAt, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("PaymentConditionRepo.List scan: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *PaymentConditionRepo) FindByID(ctx context.Context, id int64) (*domain.PaymentCondition, error) {
	var p domain.PaymentCondition
	err := r.pool.QueryRow(ctx,
		`SELECT id, code, name, days, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_payment_conditions WHERE id=$1 AND deleted_at IS NULL`, id).
		Scan(&p.ID, &p.Code, &p.Name, &p.Days, &p.DeletedAt, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("PaymentConditionRepo.FindByID: %w", err)
	}
	return &p, nil
}

func (r *PaymentConditionRepo) Create(ctx context.Context, p *domain.PaymentCondition) error {
	if p.Days < 0 {
		return fmt.Errorf("PaymentConditionRepo.Create: days must be >= 0")
	}
	err := r.pool.QueryRow(ctx,
		`INSERT INTO catalogos.catalog_payment_conditions (code, name, days)
		 VALUES ($1,$2,$3) RETURNING id, created_at, updated_at`,
		p.Code, p.Name, p.Days).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("PaymentConditionRepo.Create: %w", err)
	}
	return nil
}

func (r *PaymentConditionRepo) Update(ctx context.Context, p *domain.PaymentCondition) error {
	if p.Days < 0 {
		return fmt.Errorf("PaymentConditionRepo.Update: days must be >= 0")
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_payment_conditions SET code=$1, name=$2, days=$3
		 WHERE id=$4 AND deleted_at IS NULL`,
		p.Code, p.Name, p.Days, p.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("PaymentConditionRepo.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *PaymentConditionRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_payment_conditions SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		if isFKViolation(err) {
			return domain.ErrHasDependents
		}
		return fmt.Errorf("PaymentConditionRepo.Delete: %w", err)
	}
	return nil
}
