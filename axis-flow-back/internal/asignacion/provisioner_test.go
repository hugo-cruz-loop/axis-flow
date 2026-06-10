package asignacion_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"axis-flow-back/internal/asignacion"
)

func TestDefaultAssignmentProvisionerCreatesDeterministicActivitiesAndTools(t *testing.T) {
	activityRepo := &activityRepositorySpy{}
	toolRepo := &toolRepositorySpy{}
	clock := time.Date(2026, 6, 8, 18, 40, 0, 0, time.UTC)
	provisioner := asignacion.NewDefaultAssignmentProvisioner(activityRepo, toolRepo, fixedClock(clock))

	assignmentID := uuid.New()
	serviceID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	assignment := asignacion.Assignment{ID: assignmentID, ServiceID: serviceID}

	result, err := provisioner.ProvisionCreatedAssignment(context.Background(), assignment)
	require.NoError(t, err)
	require.GreaterOrEqual(t, result.ActivitiesCount, 2)
	require.LessOrEqual(t, result.ActivitiesCount, 4)
	require.GreaterOrEqual(t, result.ToolsCount, 1)
	require.LessOrEqual(t, result.ToolsCount, 2)
	assert.Equal(t, result.ActivitiesCount, len(activityRepo.createdActivities))
	assert.Equal(t, result.ToolsCount, len(toolRepo.createdTools))
	for _, a := range activityRepo.createdActivities {
		assert.Equal(t, assignmentID, a.AssignmentID)
		assert.Equal(t, asignacion.ActivityStatusPending, a.Status)
	}
	for _, tool := range toolRepo.createdTools {
		assert.Equal(t, assignmentID, tool.AssignmentID)
		assert.Equal(t, asignacion.ToolDeliveryStatusPending, tool.DeliveryStatus)
	}
}

func TestDefaultAssignmentProvisionerIsDeterministicPerServiceID(t *testing.T) {
	activityRepo := &activityRepositorySpy{}
	toolRepo := &toolRepositorySpy{}
	clock := time.Date(2026, 6, 8, 18, 40, 0, 0, time.UTC)
	provisioner := asignacion.NewDefaultAssignmentProvisioner(activityRepo, toolRepo, fixedClock(clock))

	serviceID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	assignmentA := asignacion.Assignment{ID: uuid.New(), ServiceID: serviceID}
	assignmentB := asignacion.Assignment{ID: uuid.New(), ServiceID: serviceID}

	first, err := provisioner.ProvisionCreatedAssignment(context.Background(), assignmentA)
	require.NoError(t, err)
	second, err := provisioner.ProvisionCreatedAssignment(context.Background(), assignmentB)
	require.NoError(t, err)

	assert.Equal(t, first.ActivitiesCount, second.ActivitiesCount)
	assert.Equal(t, first.ToolsCount, second.ToolsCount)
}

func TestDefaultAssignmentProvisionerRefreshReusesSameCounts(t *testing.T) {
	activityRepo := &activityRepositorySpy{}
	toolRepo := &toolRepositorySpy{}
	clock := time.Date(2026, 6, 8, 18, 40, 0, 0, time.UTC)
	provisioner := asignacion.NewDefaultAssignmentProvisioner(activityRepo, toolRepo, fixedClock(clock))

	assignment := asignacion.Assignment{ID: uuid.New(), ServiceID: uuid.New()}
	created, err := provisioner.ProvisionCreatedAssignment(context.Background(), assignment)
	require.NoError(t, err)
	refresh, err := provisioner.RefreshModifiedAssignment(context.Background(), assignment)
	require.NoError(t, err)

	assert.Equal(t, created.ActivitiesCount, refresh.ActivitiesCount)
	assert.Equal(t, created.ToolsCount, refresh.ToolsCount)
	assert.Equal(t, refresh.ActivitiesCount*2, len(activityRepo.createdActivities), "refresh appends a second batch")
}

func TestDefaultAssignmentProvisionerPropagatesActivityRepoError(t *testing.T) {
	activityRepo := &activityRepositorySpy{createBatchErr: errProvisionerSentinel}
	toolRepo := &toolRepositorySpy{}
	provisioner := asignacion.NewDefaultAssignmentProvisioner(activityRepo, toolRepo, fixedClock(time.Now().UTC()))

	_, err := provisioner.ProvisionCreatedAssignment(context.Background(), asignacion.Assignment{ID: uuid.New(), ServiceID: uuid.New()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "activities")
}

func TestDefaultAssignmentProvisionerPropagatesToolRepoError(t *testing.T) {
	activityRepo := &activityRepositorySpy{}
	toolRepo := &toolRepositorySpy{createBatchErr: errProvisionerSentinel}
	provisioner := asignacion.NewDefaultAssignmentProvisioner(activityRepo, toolRepo, fixedClock(time.Now().UTC()))

	_, err := provisioner.ProvisionCreatedAssignment(context.Background(), asignacion.Assignment{ID: uuid.New(), ServiceID: uuid.New()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tools")
}

var errProvisionerSentinel = errSentinelType("provisioner batch failed")

type errSentinelType string

func (e errSentinelType) Error() string { return string(e) }
