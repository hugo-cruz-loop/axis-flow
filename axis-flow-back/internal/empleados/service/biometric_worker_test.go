package service_test

import (
	"context"
	"errors"
	"testing"

	empleados "axis-flow-back/internal/empleados"
	"axis-flow-back/internal/empleados/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type fakeBiometricRepo struct {
	job       *service.BiometricJob
	jobErr    error
	result    *service.BiometricVerificationResult
	resultErr error
}

func (f *fakeBiometricRepo) ClaimNextPendingOrFailed(ctx context.Context) (*service.BiometricJob, error) {
	return f.job, f.jobErr
}

func (f *fakeBiometricRepo) SaveBiometricResult(ctx context.Context, result service.BiometricVerificationResult) error {
	f.result = &result
	return f.resultErr
}

type fakePhotoLoader struct {
	photos map[string][]byte
}

func (f fakePhotoLoader) LoadPhoto(ctx context.Context, url string) ([]byte, error) {
	photo, ok := f.photos[url]
	if !ok {
		return nil, errors.New("missing photo")
	}
	return photo, nil
}

type fakeRecognizer struct {
	result service.FaceComparisonResult
	err    error
	calls  int
}

func (f *fakeRecognizer) CompareFaces(ctx context.Context, sourceImage, targetImage []byte) (service.FaceComparisonResult, error) {
	f.calls++
	return f.result, f.err
}

type fakeBiometricPublisher struct {
	events []service.AsistenciaVerificadaEvent
}

func (f *fakeBiometricPublisher) PublishAsistenciaVerificada(ctx context.Context, event service.AsistenciaVerificadaEvent) error {
	f.events = append(f.events, event)
	return nil
}

func TestBiometricWorkerProcessOneValidatesMatchingFace(t *testing.T) {
	asistenciaID := uuid.New()
	repo := &fakeBiometricRepo{job: &service.BiometricJob{
		AsistenciaID:  asistenciaID,
		EmpleadoID:    105,
		EmpresaID:     12,
		BasePhotoURL:  "base.jpg",
		CheckPhotoURL: "check.jpg",
	}}
	recognizer := &fakeRecognizer{result: service.FaceComparisonResult{Similarity: 85.5, Passed: true}}
	publisher := &fakeBiometricPublisher{}
	worker := service.NewBiometricWorker(repo, fakePhotoLoader{photos: map[string][]byte{"base.jpg": {0x01}, "check.jpg": {0x02}}}, recognizer, publisher)

	processed, err := worker.ProcessOne(context.Background())

	require.NoError(t, err)
	require.True(t, processed)
	require.Equal(t, 1, recognizer.calls)
	require.Equal(t, empleados.ObservacionValidada, repo.result.EstatusObservacion)
	require.InDelta(t, 85.5, repo.result.SimilitudFacial, 0.001)
	require.Equal(t, asistenciaID, publisher.events[0].AsistenciaID)
	require.Equal(t, empleados.ObservacionValidada, publisher.events[0].EstatusObservacion)
}

func TestBiometricWorkerProcessOneRejectsNonMatchingFace(t *testing.T) {
	asistenciaID := uuid.New()
	repo := &fakeBiometricRepo{job: &service.BiometricJob{AsistenciaID: asistenciaID, EmpleadoID: 105, EmpresaID: 12, BasePhotoURL: "base.jpg", CheckPhotoURL: "check.jpg"}}
	recognizer := &fakeRecognizer{result: service.FaceComparisonResult{Similarity: 52.4, Passed: false}}
	publisher := &fakeBiometricPublisher{}
	worker := service.NewBiometricWorker(repo, fakePhotoLoader{photos: map[string][]byte{"base.jpg": {0x01}, "check.jpg": {0x02}}}, recognizer, publisher)

	processed, err := worker.ProcessOne(context.Background())

	require.NoError(t, err)
	require.True(t, processed)
	require.Equal(t, empleados.ObservacionRechazada, repo.result.EstatusObservacion)
	require.InDelta(t, 52.4, repo.result.SimilitudFacial, 0.001)
	require.Equal(t, empleados.ObservacionRechazada, publisher.events[0].EstatusObservacion)
}

func TestBiometricWorkerProcessOneNoWorkReturnsFalse(t *testing.T) {
	worker := service.NewBiometricWorker(&fakeBiometricRepo{}, fakePhotoLoader{}, &fakeRecognizer{}, &fakeBiometricPublisher{})

	processed, err := worker.ProcessOne(context.Background())

	require.NoError(t, err)
	require.False(t, processed)
}
