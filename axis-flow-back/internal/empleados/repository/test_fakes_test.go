package repository

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	empleados "axis-flow-back/internal/empleados"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type recordedCall struct {
	op   string
	sql  string
	args []any
}

type fakeDB struct {
	calls        []recordedCall
	execAffected []int64
	execErrs     []error
	queryRows    []rowResult
	queryResults []*fakeRows
	tx           *fakeTx
	beginCalled  bool
	beginErr     error
}

func (f *fakeDB) Begin(ctx context.Context) (dbTx, error) {
	f.beginCalled = true
	if f.beginErr != nil {
		return nil, f.beginErr
	}
	if f.tx == nil {
		f.tx = &fakeTx{}
	}
	return f.tx, nil
}

func (f *fakeDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.calls = append(f.calls, recordedCall{op: "exec", sql: sql, args: args})
	if len(f.execErrs) > 0 {
		err := f.execErrs[0]
		f.execErrs = f.execErrs[1:]
		if err != nil {
			return pgconn.CommandTag{}, err
		}
	}
	affected := int64(1)
	if len(f.execAffected) > 0 {
		affected = f.execAffected[0]
		f.execAffected = f.execAffected[1:]
	}
	return pgconn.NewCommandTag(fmt.Sprintf("UPDATE %d", affected)), nil
}

func (f *fakeDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	f.calls = append(f.calls, recordedCall{op: "queryrow", sql: sql, args: args})
	if len(f.queryRows) == 0 {
		return rowResult{err: pgx.ErrNoRows}
	}
	row := f.queryRows[0]
	f.queryRows = f.queryRows[1:]
	return row
}

func (f *fakeDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	f.calls = append(f.calls, recordedCall{op: "query", sql: sql, args: args})
	if len(f.queryResults) == 0 {
		return &fakeRows{}, nil
	}
	rows := f.queryResults[0]
	f.queryResults = f.queryResults[1:]
	return rows, rows.err
}

type fakeTx struct {
	fakeDB
	committed   bool
	rolledBack  bool
	commitErr   error
	rollbackErr error
}

func (f *fakeTx) Commit(ctx context.Context) error {
	f.committed = true
	return f.commitErr
}

func (f *fakeTx) Rollback(ctx context.Context) error {
	f.rolledBack = true
	return f.rollbackErr
}

type rowResult struct {
	values []any
	err    error
}

func (r rowResult) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return fmt.Errorf("scan expected %d destinations, got %d", len(r.values), len(dest))
	}
	for i := range dest {
		assign(dest[i], r.values[i])
	}
	return nil
}

type fakeRows struct {
	rows []rowResult
	idx  int
	err  error
}

func (f *fakeRows) Close()                                       {}
func (f *fakeRows) Err() error                                   { return f.err }
func (f *fakeRows) CommandTag() pgconn.CommandTag                { return pgconn.NewCommandTag("SELECT") }
func (f *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (f *fakeRows) Next() bool {
	if f.idx >= len(f.rows) {
		return false
	}
	f.idx++
	return true
}
func (f *fakeRows) Scan(dest ...any) error { return f.rows[f.idx-1].Scan(dest...) }
func (f *fakeRows) Values() ([]any, error) { return f.rows[f.idx-1].values, nil }
func (f *fakeRows) RawValues() [][]byte    { return nil }
func (f *fakeRows) Conn() *pgx.Conn        { return nil }

func assign(dest any, val any) {
	switch d := dest.(type) {
	case *int64:
		*d = val.(int64)
	case *int:
		*d = val.(int)
	case *string:
		*d = val.(string)
	case **string:
		if val == nil {
			*d = nil
			return
		}
		switch v := val.(type) {
		case string:
			*d = &v
		case *string:
			*d = v
		default:
			panic(fmt.Sprintf("unsupported string pointer value %T", val))
		}
	case *bool:
		*d = val.(bool)
	case *uuid.UUID:
		*d = val.(uuid.UUID)
	case **uuid.UUID:
		if val == nil {
			*d = nil
			return
		}
		switch v := val.(type) {
		case uuid.UUID:
			*d = &v
		case *uuid.UUID:
			*d = v
		default:
			panic(fmt.Sprintf("unsupported uuid pointer value %T", val))
		}
	case *time.Time:
		*d = val.(time.Time)
	case **time.Time:
		if val == nil {
			*d = nil
			return
		}
		switch v := val.(type) {
		case time.Time:
			*d = &v
		case *time.Time:
			*d = v
		default:
			panic(fmt.Sprintf("unsupported time pointer value %T", val))
		}
	case *float64:
		*d = val.(float64)
	case **float64:
		if val == nil {
			*d = nil
			return
		}
		switch v := val.(type) {
		case float64:
			*d = &v
		case *float64:
			*d = v
		default:
			panic(fmt.Sprintf("unsupported float pointer value %T", val))
		}
	case *[]byte:
		*d = val.([]byte)
	default:
		panic(fmt.Sprintf("unsupported scan destination %T", dest))
	}
}

func normalizeSQL(sql string) string {
	ws := regexp.MustCompile(`\s+`)
	return strings.TrimSpace(ws.ReplaceAllString(sql, " "))
}

func assertSQLContains(sql, want string) bool {
	return strings.Contains(strings.ToLower(normalizeSQL(sql)), strings.ToLower(want))
}

func empleadoRow(e empleados.Empleado) rowResult {
	return rowResult{values: []any{e.NumEmpleado, e.IDEmpleado, e.UsuarioID, e.EmpresaID, e.Nombre, e.ApellidoPaterno, e.ApellidoMaterno, e.Status, e.CreatedAt, e.UpdatedAt}}
}
