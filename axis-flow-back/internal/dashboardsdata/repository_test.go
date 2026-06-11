package dashboardsdata_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"axis-flow-back/internal/dashboardsdata"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRow simulates a pgx.Row for scanning single records.
type mockRow struct {
	vals []any
	err  error
}

func (m *mockRow) Scan(dest ...any) error {
	if m.err != nil {
		return m.err
	}
	if len(dest) != len(m.vals) {
		return fmt.Errorf("scan mismatch: dest len %d, vals len %d", len(dest), len(m.vals))
	}
	for i, v := range m.vals {
		if v == nil {
			continue
		}
		switch d := dest[i].(type) {
		case *int:
			*d = v.(int)
		case *int64:
			*d = v.(int64)
		case *float64:
			*d = v.(float64)
		case *string:
			*d = v.(string)
		case *bool:
			*d = v.(bool)
		case *uuid.UUID:
			*d = v.(uuid.UUID)
		case *time.Time:
			*d = v.(time.Time)
		case **int:
			if val, ok := v.(int); ok {
				*d = &val
			} else if valPtr, ok := v.(*int); ok {
				*d = valPtr
			}
		default:
			return fmt.Errorf("unsupported dest type: %T", d)
		}
	}
	return nil
}

// mockRows simulates pgx.Rows for scanning multiple records.
type mockRows struct {
	rows [][]any
	idx  int
	err  error
}

func (m *mockRows) Close()                                       {}
func (m *mockRows) Err() error                                   { return m.err }
func (m *mockRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (m *mockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (m *mockRows) Values() ([]any, error) {
	if m.idx < 1 || m.idx > len(m.rows) {
		return nil, fmt.Errorf("out of bounds values")
	}
	return m.rows[m.idx-1], nil
}
func (m *mockRows) RawValues() [][]byte { return nil }
func (m *mockRows) Conn() *pgx.Conn     { return nil }
func (m *mockRows) Next() bool {
	m.idx++
	return m.idx <= len(m.rows)
}
func (m *mockRows) Scan(dest ...any) error {
	if m.idx < 1 || m.idx > len(m.rows) {
		return fmt.Errorf("out of bounds scan")
	}
	row := m.rows[m.idx-1]
	if len(dest) != len(row) {
		return fmt.Errorf("scan mismatch: dest len %d, vals len %d", len(dest), len(row))
	}
	for i, v := range row {
		if v == nil {
			continue
		}
		switch d := dest[i].(type) {
		case *int:
			*d = v.(int)
		case *int64:
			*d = v.(int64)
		case *float64:
			*d = v.(float64)
		case *string:
			*d = v.(string)
		case *bool:
			*d = v.(bool)
		case *uuid.UUID:
			*d = v.(uuid.UUID)
		case *time.Time:
			*d = v.(time.Time)
		default:
			return fmt.Errorf("unsupported dest type: %T", d)
		}
	}
	return nil
}

// mockDB implements dashboardsdata.dbConn.
type mockDB struct {
	queryRowFunc func(sql string, args ...any) pgx.Row
	queryFunc    func(sql string, args ...any) (pgx.Rows, error)
}

func (m *mockDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.queryRowFunc != nil {
		return m.queryRowFunc(sql, args...)
	}
	return &mockRow{err: pgx.ErrNoRows}
}

func (m *mockDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.queryFunc != nil {
		return m.queryFunc(sql, args...)
	}
	return &mockRows{}, nil
}

func TestPgxRepository_GetEvaluacionesClientes(t *testing.T) {
	clientID := uuid.New()
	db := &mockDB{
		queryFunc: func(sql string, args ...any) (pgx.Rows, error) {
			assert.Contains(t, sql, "clientes.clientes_cliente")
			assert.Equal(t, int64(1), args[0])
			return &mockRows{
				rows: [][]any{
					{clientID, "Client A", 4.5, 10},
				},
			}, nil
		},
	}
	repo := dashboardsdata.NewPgxRepository(db)
	res, err := repo.GetEvaluacionesClientes(context.Background(), 1, nil)
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, clientID, res[0].ClientID)
	assert.Equal(t, "Client A", res[0].ClientNombre)
	assert.Equal(t, 4.5, res[0].PromedioPuntuacion)
	assert.Equal(t, 10, res[0].TotalEvaluaciones)
}

func TestPgxRepository_GetEvaluacionesClientes_WithFilter(t *testing.T) {
	clientID := uuid.New()
	db := &mockDB{
		queryFunc: func(sql string, args ...any) (pgx.Rows, error) {
			assert.Contains(t, sql, "clientes.clientes_cliente")
			assert.Contains(t, sql, "AND c.id = $2")
			assert.Equal(t, int64(1), args[0])
			assert.Equal(t, clientID, args[1])
			return &mockRows{
				rows: [][]any{
					{clientID, "Client A", 4.5, 10},
				},
			}, nil
		},
	}
	repo := dashboardsdata.NewPgxRepository(db)
	res, err := repo.GetEvaluacionesClientes(context.Background(), 1, &clientID)
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, clientID, res[0].ClientID)
}

func TestPgxRepository_GetEmpleadosCount(t *testing.T) {
	db := &mockDB{
		queryRowFunc: func(sql string, args ...any) pgx.Row {
			assert.Contains(t, sql, "empleados.empleados_empleado")
			assert.Equal(t, int64(1), args[0])
			return &mockRow{vals: []any{42}}
		},
	}
	repo := dashboardsdata.NewPgxRepository(db)
	count, err := repo.GetEmpleadosCount(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, 42, count)
}

func TestPgxRepository_GetActividadesCount(t *testing.T) {
	since := time.Now().Add(-24 * time.Hour)
	db := &mockDB{
		queryRowFunc: func(sql string, args ...any) pgx.Row {
			assert.Contains(t, sql, "asignacion.asignacion_asignaactividad")
			assert.Equal(t, int64(1), args[0])
			assert.Equal(t, since, args[1])
			return &mockRow{vals: []any{15}}
		},
	}
	repo := dashboardsdata.NewPgxRepository(db)
	count, err := repo.GetActividadesCount(context.Background(), 1, since)
	require.NoError(t, err)
	assert.Equal(t, 15, count)
}

func TestPgxRepository_GetAusenciasCount(t *testing.T) {
	year := 2026
	db := &mockDB{
		queryRowFunc: func(sql string, args ...any) pgx.Row {
			assert.Contains(t, sql, "empleados.empleados_inasistencia")
			assert.Equal(t, int64(1), args[0])
			assert.Equal(t, year, args[1])
			return &mockRow{vals: []any{5}}
		},
	}
	repo := dashboardsdata.NewPgxRepository(db)
	count, err := repo.GetAusenciasCount(context.Background(), 1, &year)
	require.NoError(t, err)
	assert.Equal(t, 5, count)
}

func TestPgxRepository_GetServiciosLocalidad(t *testing.T) {
	clientID := uuid.New()
	localidadID := uuid.New()
	db := &mockDB{
		queryFunc: func(sql string, args ...any) (pgx.Rows, error) {
			assert.Contains(t, sql, "clientes.clientes_localidad")
			assert.Equal(t, clientID, args[0])
			return &mockRows{
				rows: [][]any{
					{localidadID, "Locality 1", int64(10), "Service A", 3},
				},
			}, nil
		},
	}
	repo := dashboardsdata.NewPgxRepository(db)
	res, err := repo.GetServiciosLocalidad(context.Background(), clientID)
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, localidadID, res[0].LocalidadID)
	assert.Equal(t, "Locality 1", res[0].LocalidadNombre)
	require.Len(t, res[0].Servicios, 1)
	assert.Equal(t, int64(10), res[0].Servicios[0].ServicioID)
	assert.Equal(t, "Service A", res[0].Servicios[0].ServicioNombre)
	assert.Equal(t, 3, res[0].Servicios[0].EmpleadosAsignados)
}

func TestPgxRepository_GetAtencionSeguimientoStatus(t *testing.T) {
	clientID := uuid.New()
	db := &mockDB{
		queryRowFunc: func(sql string, args ...any) pgx.Row {
			assert.Contains(t, sql, "atencion_seguimiento.tickets_servicio")
			assert.Equal(t, clientID, args[0])
			return &mockRow{vals: []any{2, 3, 5, 10}}
		},
	}
	repo := dashboardsdata.NewPgxRepository(db)
	tickets, total, err := repo.GetAtencionSeguimientoStatus(context.Background(), clientID)
	require.NoError(t, err)
	assert.Equal(t, 10, total)
	assert.Equal(t, 2, tickets.Pendiente)
	assert.Equal(t, 3, tickets.EnProceso)
	assert.Equal(t, 5, tickets.Finalizado)
}

func TestPgxRepository_GetBolsaTrabajoVacantesActivas(t *testing.T) {
	companyTenantID := uuid.New()
	now := time.Now()
	db := &mockDB{
		queryFunc: func(sql string, args ...any) (pgx.Rows, error) {
			assert.Contains(t, sql, "bolsa_trabajo.trabajos")
			assert.Equal(t, companyTenantID, args[0])
			return &mockRows{
				rows: [][]any{
					{uuid.New().String(), "Go Dev", now},
				},
			}, nil
		},
	}
	repo := dashboardsdata.NewPgxRepository(db)
	res, err := repo.GetBolsaTrabajoVacantesActivas(context.Background(), companyTenantID)
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, "Go Dev", res[0].Titulo)
	assert.Equal(t, "Operaciones", res[0].Departamento)
}
