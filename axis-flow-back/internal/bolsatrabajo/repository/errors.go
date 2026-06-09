package repository

import (
	"errors"
	"fmt"

	"axis-flow-back/internal/bolsatrabajo"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
)

// mapError maps pgconn and pgx errors to bolsatrabajo domain sentinels.
// op is the caller name used as the error prefix.
func mapError(err error, op string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" {
			return fmt.Errorf("%s: %w", op, bolsatrabajo.ErrConflict)
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%s: %w", op, bolsatrabajo.ErrNotFound)
	}
	return fmt.Errorf("%s: %w", op, err)
}
