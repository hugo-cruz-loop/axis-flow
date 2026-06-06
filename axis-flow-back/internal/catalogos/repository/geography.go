package repository

import (
	"context"
	"errors"
	"fmt"

	"axis-flow-back/internal/catalogos/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CountryRepo handles catalogos.catalog_countries.
type CountryRepo struct{ pool *pgxpool.Pool }

func NewCountryRepo(pool *pgxpool.Pool) *CountryRepo { return &CountryRepo{pool: pool} }

func (r *CountryRepo) List(ctx context.Context) ([]domain.Country, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, code, name, phone_code, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_countries
		 WHERE deleted_at IS NULL
		 ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("CountryRepo.List: %w", err)
	}
	defer rows.Close()

	var out []domain.Country
	for rows.Next() {
		var c domain.Country
		if err := rows.Scan(&c.ID, &c.Code, &c.Name, &c.PhoneCode, &c.DeletedAt, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("CountryRepo.List scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *CountryRepo) FindByID(ctx context.Context, id int64) (*domain.Country, error) {
	var c domain.Country
	err := r.pool.QueryRow(ctx,
		`SELECT id, code, name, phone_code, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_countries WHERE id=$1 AND deleted_at IS NULL`, id).
		Scan(&c.ID, &c.Code, &c.Name, &c.PhoneCode, &c.DeletedAt, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("CountryRepo.FindByID: %w", err)
	}
	return &c, nil
}

func (r *CountryRepo) Create(ctx context.Context, c *domain.Country) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO catalogos.catalog_countries (code, name, phone_code)
		 VALUES ($1,$2,$3) RETURNING id, created_at, updated_at`,
		c.Code, c.Name, c.PhoneCode).
		Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("CountryRepo.Create: %w", err)
	}
	return nil
}

func (r *CountryRepo) Update(ctx context.Context, c *domain.Country) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_countries SET code=$1, name=$2, phone_code=$3
		 WHERE id=$4 AND deleted_at IS NULL`,
		c.Code, c.Name, c.PhoneCode, c.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("CountryRepo.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *CountryRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_countries SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		if isFKViolation(err) {
			return domain.ErrHasDependents
		}
		return fmt.Errorf("CountryRepo.Delete: %w", err)
	}
	return nil
}

// StateRepo handles catalogos.catalog_states.
type StateRepo struct{ pool *pgxpool.Pool }

func NewStateRepo(pool *pgxpool.Pool) *StateRepo { return &StateRepo{pool: pool} }

func (r *StateRepo) ListByCountryID(ctx context.Context, countryID int64) ([]domain.State, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, country_id, code, name, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_states
		 WHERE country_id=$1 AND deleted_at IS NULL
		 ORDER BY name ASC`, countryID)
	if err != nil {
		return nil, fmt.Errorf("StateRepo.ListByCountryID: %w", err)
	}
	defer rows.Close()

	var out []domain.State
	for rows.Next() {
		var s domain.State
		if err := rows.Scan(&s.ID, &s.CountryID, &s.Code, &s.Name, &s.DeletedAt, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("StateRepo.ListByCountryID scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *StateRepo) FindByID(ctx context.Context, id int64) (*domain.State, error) {
	var s domain.State
	err := r.pool.QueryRow(ctx,
		`SELECT id, country_id, code, name, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_states WHERE id=$1 AND deleted_at IS NULL`, id).
		Scan(&s.ID, &s.CountryID, &s.Code, &s.Name, &s.DeletedAt, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("StateRepo.FindByID: %w", err)
	}
	return &s, nil
}

func (r *StateRepo) Create(ctx context.Context, s *domain.State) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO catalogos.catalog_states (country_id, code, name)
		 VALUES ($1,$2,$3) RETURNING id, created_at, updated_at`,
		s.CountryID, s.Code, s.Name).
		Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		if isFKViolation(err) {
			return domain.ErrHasDependents
		}
		return fmt.Errorf("StateRepo.Create: %w", err)
	}
	return nil
}

func (r *StateRepo) Update(ctx context.Context, s *domain.State) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_states SET country_id=$1, code=$2, name=$3
		 WHERE id=$4 AND deleted_at IS NULL`,
		s.CountryID, s.Code, s.Name, s.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("StateRepo.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *StateRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_states SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		if isFKViolation(err) {
			return domain.ErrHasDependents
		}
		return fmt.Errorf("StateRepo.Delete: %w", err)
	}
	return nil
}

// CityRepo handles catalogos.catalog_cities.
type CityRepo struct{ pool *pgxpool.Pool }

func NewCityRepo(pool *pgxpool.Pool) *CityRepo { return &CityRepo{pool: pool} }

func (r *CityRepo) List(ctx context.Context) ([]domain.City, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, state_id, name, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_cities
		 WHERE deleted_at IS NULL
		 ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("CityRepo.List: %w", err)
	}
	defer rows.Close()

	var out []domain.City
	for rows.Next() {
		var c domain.City
		if err := rows.Scan(&c.ID, &c.StateID, &c.Name, &c.DeletedAt, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("CityRepo.List scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *CityRepo) ListByStateID(ctx context.Context, stateID int64) ([]domain.City, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, state_id, name, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_cities
		 WHERE state_id=$1 AND deleted_at IS NULL
		 ORDER BY name ASC`, stateID)
	if err != nil {
		return nil, fmt.Errorf("CityRepo.ListByStateID: %w", err)
	}
	defer rows.Close()

	var out []domain.City
	for rows.Next() {
		var c domain.City
		if err := rows.Scan(&c.ID, &c.StateID, &c.Name, &c.DeletedAt, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("CityRepo.ListByStateID scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *CityRepo) FindByID(ctx context.Context, id int64) (*domain.City, error) {
	var c domain.City
	err := r.pool.QueryRow(ctx,
		`SELECT id, state_id, name, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_cities WHERE id=$1 AND deleted_at IS NULL`, id).
		Scan(&c.ID, &c.StateID, &c.Name, &c.DeletedAt, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("CityRepo.FindByID: %w", err)
	}
	return &c, nil
}

func (r *CityRepo) Create(ctx context.Context, c *domain.City) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO catalogos.catalog_cities (state_id, name)
		 VALUES ($1,$2) RETURNING id, created_at, updated_at`,
		c.StateID, c.Name).
		Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		if isFKViolation(err) {
			return domain.ErrHasDependents
		}
		return fmt.Errorf("CityRepo.Create: %w", err)
	}
	return nil
}

func (r *CityRepo) Update(ctx context.Context, c *domain.City) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_cities SET state_id=$1, name=$2
		 WHERE id=$3 AND deleted_at IS NULL`,
		c.StateID, c.Name, c.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("CityRepo.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *CityRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE catalogos.catalog_cities SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		if isFKViolation(err) {
			return domain.ErrHasDependents
		}
		return fmt.Errorf("CityRepo.Delete: %w", err)
	}
	return nil
}

// LocalityTypeRepo handles catalogos.catalog_locality_types.
type LocalityTypeRepo struct{ pool *pgxpool.Pool }

func NewLocalityTypeRepo(pool *pgxpool.Pool) *LocalityTypeRepo {
	return &LocalityTypeRepo{pool: pool}
}

func (r *LocalityTypeRepo) List(ctx context.Context) ([]domain.LocalityType, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, code, name, deleted_at, created_at, updated_at
		 FROM catalogos.catalog_locality_types
		 WHERE deleted_at IS NULL
		 ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("LocalityTypeRepo.List: %w", err)
	}
	defer rows.Close()

	var out []domain.LocalityType
	for rows.Next() {
		var lt domain.LocalityType
		if err := rows.Scan(&lt.ID, &lt.Code, &lt.Name, &lt.DeletedAt, &lt.CreatedAt, &lt.UpdatedAt); err != nil {
			return nil, fmt.Errorf("LocalityTypeRepo.List scan: %w", err)
		}
		out = append(out, lt)
	}
	return out, rows.Err()
}

func (r *LocalityTypeRepo) Create(ctx context.Context, lt *domain.LocalityType) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO catalogos.catalog_locality_types (code, name)
		 VALUES ($1,$2) RETURNING id, created_at, updated_at`,
		lt.Code, lt.Name).
		Scan(&lt.ID, &lt.CreatedAt, &lt.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCode
		}
		return fmt.Errorf("LocalityTypeRepo.Create: %w", err)
	}
	return nil
}
