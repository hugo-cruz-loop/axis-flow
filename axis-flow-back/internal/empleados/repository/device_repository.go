package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	empleados "axis-flow-back/internal/empleados"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const deviceCacheTTL = 7 * 24 * time.Hour

// DeviceRepository persists authorized employee devices with a Redis cache layer.
type DeviceRepository struct {
	db  dbConn
	rdb *redis.Client
}

func NewDeviceRepository(db dbConn, rdb *redis.Client) *DeviceRepository {
	return &DeviceRepository{db: db, rdb: rdb}
}
func NewPgxDeviceRepository(pool *pgxpool.Pool, rdb *redis.Client) *DeviceRepository {
	return NewDeviceRepository(pgxDB{pool: pool}, rdb)
}

func (r *DeviceRepository) Upsert(ctx context.Context, d *empleados.UserDevice, empresaID int64) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO empleados.empleados_user_devices
            (empleado_id, device_uuid, device_model, os_version, fcm_token, is_active)
         SELECT $1,$2,$3,$4,$5,true
         WHERE EXISTS (SELECT 1 FROM empleados.empleados_empleado e WHERE e.num_empleado=$1 AND e.empresa_id=$6)
         ON CONFLICT (empleado_id, device_uuid) DO UPDATE SET
            device_model=EXCLUDED.device_model, os_version=EXCLUDED.os_version, fcm_token=EXCLUDED.fcm_token,
            is_active=true, updated_at=NOW()
         RETURNING id, is_active, created_at, updated_at`,
		d.EmpleadoID, d.DeviceUUID, d.DeviceModel, d.OSVersion, d.FCMToken, empresaID).
		Scan(&d.ID, &d.IsActive, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return fmt.Errorf("deviceRepository.Upsert: %w", err)
	}
	if err := r.cacheDevice(ctx, *d); err != nil {
		return fmt.Errorf("deviceRepository.Upsert cache: %w", err)
	}
	return nil
}

func (r *DeviceRepository) GetByEmpleado(ctx context.Context, empleadoID, empresaID int64) ([]empleados.UserDevice, error) {
	rows, err := r.db.Query(ctx, deviceSelect()+` WHERE d.empleado_id=$1 AND e.empresa_id=$2 ORDER BY d.updated_at DESC`, empleadoID, empresaID)
	if err != nil {
		return nil, fmt.Errorf("deviceRepository.GetByEmpleado: %w", err)
	}
	defer rows.Close()
	devices, err := scanDeviceRows(rows)
	if err != nil {
		return nil, err
	}
	for _, d := range devices {
		_ = r.cacheDevice(ctx, d)
	}
	return devices, nil
}

func (r *DeviceRepository) GetByDevice(ctx context.Context, empleadoID, empresaID int64, deviceUUID string) (*empleados.UserDevice, error) {
	if r.rdb != nil {
		payload, err := r.rdb.Get(ctx, deviceCacheKey(empleadoID, deviceUUID)).Bytes()
		if err == nil {
			d, decodeErr := decodeDevice(payload)
			if decodeErr != nil {
				return nil, fmt.Errorf("deviceRepository.GetByDevice cache decode: %w", decodeErr)
			}
			return d, nil
		}
		if err != nil && !errors.Is(err, redis.Nil) {
			return nil, fmt.Errorf("deviceRepository.GetByDevice cache: %w", err)
		}
	}

	d, err := scanDevice(r.db.QueryRow(ctx, deviceSelect()+` WHERE d.empleado_id=$1 AND d.device_uuid=$2 AND e.empresa_id=$3`, empleadoID, deviceUUID, empresaID))
	if err != nil {
		return nil, err
	}
	if err := r.cacheDevice(ctx, *d); err != nil {
		return nil, fmt.Errorf("deviceRepository.GetByDevice cache set: %w", err)
	}
	return d, nil
}

func (r *DeviceRepository) cacheDevice(ctx context.Context, d empleados.UserDevice) error {
	if r.rdb == nil {
		return nil
	}
	payload, err := encodeDevice(d)
	if err != nil {
		return err
	}
	return r.rdb.Set(ctx, deviceCacheKey(d.EmpleadoID, d.DeviceUUID), payload, deviceCacheTTL).Err()
}

func deviceCacheKey(empleadoID int64, deviceUUID string) string {
	return fmt.Sprintf("empleados:dispositivo:%d:%s", empleadoID, deviceUUID)
}

type cachedDevice struct {
	ID          uuid.UUID `json:"id"`
	EmpleadoID  int64     `json:"empleado_id"`
	DeviceUUID  string    `json:"device_uuid"`
	DeviceModel *string   `json:"device_model,omitempty"`
	OSVersion   *string   `json:"os_version,omitempty"`
	FCMToken    string    `json:"fcm_token"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func encodeDevice(d empleados.UserDevice) ([]byte, error) {
	return json.Marshal(cachedDevice{ID: d.ID, EmpleadoID: d.EmpleadoID, DeviceUUID: d.DeviceUUID, DeviceModel: d.DeviceModel, OSVersion: d.OSVersion, FCMToken: d.FCMToken, IsActive: d.IsActive, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt})
}

func decodeDevice(payload []byte) (*empleados.UserDevice, error) {
	var c cachedDevice
	if err := json.Unmarshal(payload, &c); err != nil {
		return nil, err
	}
	return &empleados.UserDevice{ID: c.ID, EmpleadoID: c.EmpleadoID, DeviceUUID: c.DeviceUUID, DeviceModel: c.DeviceModel, OSVersion: c.OSVersion, FCMToken: c.FCMToken, IsActive: c.IsActive, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}, nil
}

func deviceSelect() string {
	return `SELECT d.id, d.empleado_id, d.device_uuid, d.device_model, d.os_version, d.fcm_token,
                   d.is_active, d.created_at, d.updated_at
            FROM empleados.empleados_user_devices d
            JOIN empleados.empleados_empleado e ON e.num_empleado=d.empleado_id`
}

func scanDevice(row pgx.Row) (*empleados.UserDevice, error) {
	var d empleados.UserDevice
	err := row.Scan(&d.ID, &d.EmpleadoID, &d.DeviceUUID, &d.DeviceModel, &d.OSVersion, &d.FCMToken, &d.IsActive, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, empleados.ErrDeviceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("deviceRepository scan: %w", err)
	}
	return &d, nil
}

func scanDeviceRows(rows pgx.Rows) ([]empleados.UserDevice, error) {
	out := []empleados.UserDevice{}
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("deviceRepository rows: %w", err)
	}
	return out, nil
}
