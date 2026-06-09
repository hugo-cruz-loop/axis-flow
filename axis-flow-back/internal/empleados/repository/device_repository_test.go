package repository

import (
	"context"
	"testing"
	"time"

	"axis-flow-back/internal/empleados"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	return mr, redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func deviceRow(d empleados.UserDevice) rowResult {
	return rowResult{values: []any{d.ID, d.EmpleadoID, d.DeviceUUID, d.DeviceModel, d.OSVersion, d.FCMToken, d.IsActive, d.CreatedAt, d.UpdatedAt}}
}

func TestDeviceRepositoryUpsertStoresRedisCacheWithSevenDayTTL(t *testing.T) {
	mr, rdb := testRedis(t)
	now := time.Now()
	deviceID := uuid.New()
	db := &fakeDB{queryRows: []rowResult{{values: []any{deviceID, true, now, now}}}}
	repo := NewDeviceRepository(db, rdb)
	model := "Pixel 8"
	os := "Android 15"
	device := &empleados.UserDevice{EmpleadoID: 105, DeviceUUID: "device-uuid", DeviceModel: &model, OSVersion: &os, FCMToken: "fcm-secret"}

	err := repo.Upsert(context.Background(), device, 12)

	require.NoError(t, err)
	assert.Equal(t, deviceID, device.ID)
	key := "empleados:dispositivo:105:device-uuid"
	assert.True(t, mr.Exists(key))
	assert.Equal(t, 7*24*time.Hour, mr.TTL(key))
	assert.True(t, assertSQLContains(db.calls[0].sql, "empresa_id=$"), db.calls[0].sql)
}

func TestDeviceRepositoryGetByDeviceUsesCacheHitWithoutDatabase(t *testing.T) {
	_, rdb := testRedis(t)
	repo := NewDeviceRepository(&fakeDB{}, rdb)
	cached := empleados.UserDevice{ID: uuid.New(), EmpleadoID: 105, DeviceUUID: "device-uuid", FCMToken: "fcm-secret", IsActive: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, repo.cacheDevice(context.Background(), cached))

	got, err := repo.GetByDevice(context.Background(), 105, 12, "device-uuid")

	require.NoError(t, err)
	assert.Equal(t, cached.ID, got.ID)
	assert.Equal(t, "fcm-secret", got.FCMToken)
}

func TestDeviceRepositoryGetByDeviceCacheMissQueriesDatabaseThenCaches(t *testing.T) {
	mr, rdb := testRedis(t)
	now := time.Now()
	deviceID := uuid.New()
	db := &fakeDB{queryRows: []rowResult{deviceRow(empleados.UserDevice{ID: deviceID, EmpleadoID: 105, DeviceUUID: "device-uuid", FCMToken: "fcm-secret", IsActive: true, CreatedAt: now, UpdatedAt: now})}}
	repo := NewDeviceRepository(db, rdb)

	got, err := repo.GetByDevice(context.Background(), 105, 12, "device-uuid")

	require.NoError(t, err)
	assert.Equal(t, deviceID, got.ID)
	assert.True(t, assertSQLContains(db.calls[0].sql, "device_uuid=$2"), db.calls[0].sql)
	assert.True(t, assertSQLContains(db.calls[0].sql, "empresa_id=$3"), db.calls[0].sql)
	assert.True(t, mr.Exists("empleados:dispositivo:105:device-uuid"))
}

func TestDeviceRepositoryGetByEmpleadoReturnsEmpresaScopedDevices(t *testing.T) {
	_, rdb := testRedis(t)
	now := time.Now()
	db := &fakeDB{queryResults: []*fakeRows{{rows: []rowResult{deviceRow(empleados.UserDevice{ID: uuid.New(), EmpleadoID: 105, DeviceUUID: "d1", FCMToken: "t1", IsActive: true, CreatedAt: now, UpdatedAt: now})}}}}
	repo := NewDeviceRepository(db, rdb)

	got, err := repo.GetByEmpleado(context.Background(), 105, 12)

	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.True(t, assertSQLContains(db.calls[0].sql, "empleado_id=$1"), db.calls[0].sql)
	assert.True(t, assertSQLContains(db.calls[0].sql, "empresa_id=$2"), db.calls[0].sql)
}
