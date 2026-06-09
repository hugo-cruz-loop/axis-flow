// Package repository implements persistence for the empleados module.
package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	empleados "axis-flow-back/internal/empleados"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type dbConn interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type dbTx interface {
	dbConn
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type dbTransactor interface {
	dbConn
	Begin(ctx context.Context) (dbTx, error)
}

type pgxDB struct{ pool *pgxpool.Pool }

func (p pgxDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return p.pool.Exec(ctx, sql, args...)
}

func (p pgxDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return p.pool.Query(ctx, sql, args...)
}

func (p pgxDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return p.pool.QueryRow(ctx, sql, args...)
}

func (p pgxDB) Begin(ctx context.Context) (dbTx, error) {
	return p.pool.BeginTx(ctx, pgx.TxOptions{})
}

func rowsAffected(tag pgconn.CommandTag) int64 { return tag.RowsAffected() }

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func uniqueConstraintName(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return strings.ToLower(pgErr.ConstraintName)
	}
	return ""
}

func mapUbicacionUniqueError(err error) error {
	if !isUniqueViolation(err) {
		return err
	}
	switch uniqueConstraintName(err) {
	case "uq_empleados_ubicacion_curp":
		return empleados.ErrDuplicateCURP
	case "uq_empleados_ubicacion_nss":
		return empleados.ErrDuplicateNSS
	default:
		return fmt.Errorf("unique constraint violation: %w", err)
	}
}
