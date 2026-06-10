package events_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"axis-flow-back/internal/asignacion"
	"axis-flow-back/internal/asignacion/events"
)

func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return mr, client
}

func TestRedisStreamsAssignmentEventPublisherPublishesAsignacionModificada(t *testing.T) {
	_, client := newTestRedis(t)
	clock := time.Date(2026, 6, 8, 18, 45, 0, 0, time.UTC)
	pub := events.NewRedisStreamsAssignmentEventPublisher(client, events.RedisStreamsConfig{StreamKey: "asignacion:events"})

	assignmentID := uuid.MustParse("c7a85f64-5717-4562-b3fc-2c963f66afb0")
	employeeID := uuid.MustParse("3fa85f64-5717-4562-b3fc-2c963f66afa6")
	locationID := uuid.MustParse("3fa85f64-5717-4562-b3fc-2c963f66afa7")
	event := asignacion.AsignacionModificadaEvent{
		AssignmentID: assignmentID,
		EmployeeID:   employeeID,
		LocationID:   locationID,
		UpdatedAt:    clock,
	}

	require.NoError(t, pub.PublishAsignacionModificada(context.Background(), event))

	entries, err := client.XRange(context.Background(), "asignacion:events", "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "AsignacionModificada", entries[0].Values["event"])
	assert.NotEmpty(t, entries[0].Values["published_at"])

	var decoded asignacion.AsignacionModificadaEvent
	require.NoError(t, json.Unmarshal([]byte(entries[0].Values["payload"].(string)), &decoded))
	assert.Equal(t, assignmentID, decoded.AssignmentID)
	assert.Equal(t, employeeID, decoded.EmployeeID)
	assert.Equal(t, locationID, decoded.LocationID)
	assert.True(t, decoded.UpdatedAt.Equal(clock))
}

func TestRedisStreamsAssignmentEventPublisherPublishesEvidenciaCargada(t *testing.T) {
	_, client := newTestRedis(t)
	pub := events.NewRedisStreamsAssignmentEventPublisher(client, events.RedisStreamsConfig{StreamKey: "asignacion:events"})

	activityID := uuid.New()
	assignmentID := uuid.New()
	employeeID := uuid.New()
	uploadedAt := time.Date(2026, 6, 8, 18, 42, 0, 0, time.UTC)
	event := asignacion.EvidenciaCargadaEvent{
		ActivityID:     activityID,
		AssignmentID:   assignmentID,
		EmployeeID:     employeeID,
		EvidenceURLs:   []string{"https://checkon-evidences.s3.amazonaws.com/evidencia/1_abc.jpg"},
		GPSCoordinates: asignacion.GPSCoordinate{Latitude: 19.4326, Longitude: -99.1332},
		UploadedAt:     uploadedAt,
	}

	require.NoError(t, pub.PublishEvidenciaCargada(context.Background(), event))

	entries, err := client.XRange(context.Background(), "asignacion:events", "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "EvidenciaCargada", entries[0].Values["event"])

	var decoded asignacion.EvidenciaCargadaEvent
	require.NoError(t, json.Unmarshal([]byte(entries[0].Values["payload"].(string)), &decoded))
	assert.Equal(t, activityID, decoded.ActivityID)
	assert.Equal(t, assignmentID, decoded.AssignmentID)
	assert.Equal(t, employeeID, decoded.EmployeeID)
	require.Len(t, decoded.EvidenceURLs, 1)
	assert.InDelta(t, 19.4326, decoded.GPSCoordinates.Latitude, 1e-9)
	assert.True(t, decoded.UploadedAt.Equal(uploadedAt))
}

func TestRedisStreamsAssignmentEventPublisherUsesConfiguredStreamKey(t *testing.T) {
	mr, client := newTestRedis(t)
	pub := events.NewRedisStreamsAssignmentEventPublisher(client, events.RedisStreamsConfig{StreamKey: "custom:asignacion:events"})

	require.NoError(t, pub.PublishAsignacionModificada(context.Background(), asignacion.AsignacionModificadaEvent{
		AssignmentID: uuid.New(),
		EmployeeID:   uuid.New(),
		UpdatedAt:    time.Now().UTC(),
	}))

	assert.True(t, mr.Exists("custom:asignacion:events"))
	assert.False(t, mr.Exists("asignacion:events"))
}

func TestRedisStreamsAssignmentEventPublisherAppliesMaxLenCap(t *testing.T) {
	_, client := newTestRedis(t)
	pub := events.NewRedisStreamsAssignmentEventPublisher(client, events.RedisStreamsConfig{
		StreamKey: "asignacion:events",
		MaxLen:    3,
	})

	for i := 0; i < 5; i++ {
		require.NoError(t, pub.PublishAsignacionModificada(context.Background(), asignacion.AsignacionModificadaEvent{
			AssignmentID: uuid.New(),
			EmployeeID:   uuid.New(),
			UpdatedAt:    time.Now().UTC(),
		}))
	}

	length, err := client.XLen(context.Background(), "asignacion:events").Result()
	require.NoError(t, err)
	// XAdd with Approx MaxLen may keep MaxLen+1 entries transiently.
	assert.LessOrEqual(t, length, int64(4))
}

func TestRedisStreamsAssignmentEventPublisherReturnsErrorOnClientFailure(t *testing.T) {
	mr, client := newTestRedis(t)
	pub := events.NewRedisStreamsAssignmentEventPublisher(client, events.RedisStreamsConfig{StreamKey: "asignacion:events"})

	mr.Close()

	err := pub.PublishAsignacionModificada(context.Background(), asignacion.AsignacionModificadaEvent{
		AssignmentID: uuid.New(),
		EmployeeID:   uuid.New(),
		UpdatedAt:    time.Now().UTC(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AsignacionModificada")
}

func TestRedisStreamsAssignmentEventPublisherAppliesDefaultsWhenConfigEmpty(t *testing.T) {
	mr, client := newTestRedis(t)
	pub := events.NewRedisStreamsAssignmentEventPublisher(client, events.RedisStreamsConfig{})

	require.NoError(t, pub.PublishEvidenciaCargada(context.Background(), asignacion.EvidenciaCargadaEvent{
		ActivityID:   uuid.New(),
		AssignmentID: uuid.New(),
		EmployeeID:   uuid.New(),
		UploadedAt:   time.Now().UTC(),
	}))

	assert.True(t, mr.Exists("asignacion:events"))
}

func TestRedisStreamsAssignmentEventPublisherPublishesEmpleadoEvaluado(t *testing.T) {
	_, client := newTestRedis(t)
	pub := events.NewRedisStreamsAssignmentEventPublisher(client, events.RedisStreamsConfig{StreamKey: "asignacion:events"})

	evaluationID := uuid.New()
	assignmentID := uuid.New()
	employeeID := uuid.New()
	evaluatorID := uuid.New()
	evalDate := time.Date(2026, 6, 8, 18, 46, 0, 0, time.UTC)
	require.NoError(t, pub.PublishEmpleadoEvaluado(context.Background(), asignacion.EmpleadoEvaluadoEvent{
		EvaluationID:   evaluationID,
		AssignmentID:   assignmentID,
		EmployeeID:     employeeID,
		EvaluatorID:    evaluatorID,
		Rating:         4,
		EvaluationDate: evalDate,
	}))

	entries, err := client.XRange(context.Background(), "asignacion:events", "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "EmpleadoEvaluado", entries[0].Values["event"])

	var decoded asignacion.EmpleadoEvaluadoEvent
	require.NoError(t, json.Unmarshal([]byte(entries[0].Values["payload"].(string)), &decoded))
	assert.Equal(t, evaluationID, decoded.EvaluationID)
	assert.Equal(t, assignmentID, decoded.AssignmentID)
	assert.Equal(t, employeeID, decoded.EmployeeID)
	assert.Equal(t, evaluatorID, decoded.EvaluatorID)
	assert.Equal(t, 4, decoded.Rating)
	assert.True(t, decoded.EvaluationDate.Equal(evalDate))
}
