package service

import (
	"context"
	"fmt"
	"time"

	empleados "axis-flow-back/internal/empleados"

	"github.com/google/uuid"
)

// BiometricJob is one pending or failed attendance photo comparison to process.
type BiometricJob struct {
	AsistenciaID  uuid.UUID
	EmpleadoID    int64
	EmpresaID     int64
	BasePhotoURL  string
	CheckPhotoURL string
}

// BiometricVerificationResult is persisted after a biometric comparison finishes.
type BiometricVerificationResult struct {
	AsistenciaID       uuid.UUID
	EmpleadoID         int64
	EmpresaID          int64
	EstatusObservacion int
	SimilitudFacial    float64
	VerifiedAt         time.Time
}

// AsistenciaVerificadaEvent is emitted after a biometric check is stored.
type AsistenciaVerificadaEvent struct {
	AsistenciaID       uuid.UUID
	EmpleadoID         int64
	EmpresaID          int64
	SimilitudFacial    float64
	EstatusObservacion int
	Timestamp          time.Time
}

// BiometricWorkRepository is the service-level port for pending/failed biometric attendance work.
type BiometricWorkRepository interface {
	ClaimNextPendingOrFailed(ctx context.Context) (*BiometricJob, error)
	SaveBiometricResult(ctx context.Context, result BiometricVerificationResult) error
}

// PhotoLoader loads photo bytes from storage URLs without coupling the worker to a storage implementation.
type PhotoLoader interface {
	LoadPhoto(ctx context.Context, url string) ([]byte, error)
}

// BiometricEventPublisher publishes biometric verification events for downstream realtime gateways.
type BiometricEventPublisher interface {
	PublishAsistenciaVerificada(ctx context.Context, event AsistenciaVerificadaEvent) error
}

// BiometricWorker processes one attendance biometric verification job at a time.
type BiometricWorker struct {
	repo       BiometricWorkRepository
	photos     PhotoLoader
	recognizer FaceComparator
	publisher  BiometricEventPublisher
	clock      func() time.Time
}

// NewBiometricWorker creates a biometric worker using service-level ports.
func NewBiometricWorker(repo BiometricWorkRepository, photos PhotoLoader, recognizer FaceComparator, publisher BiometricEventPublisher) *BiometricWorker {
	return &BiometricWorker{repo: repo, photos: photos, recognizer: recognizer, publisher: publisher, clock: time.Now}
}

// ProcessOne claims and processes one pending/failed biometric job. It returns false when no work exists.
func (w *BiometricWorker) ProcessOne(ctx context.Context) (bool, error) {
	job, err := w.repo.ClaimNextPendingOrFailed(ctx)
	if err != nil {
		return false, fmt.Errorf("biometricWorker claim: %w", err)
	}
	if job == nil {
		return false, nil
	}

	basePhoto, err := w.photos.LoadPhoto(ctx, job.BasePhotoURL)
	if err != nil {
		return true, fmt.Errorf("biometricWorker load base photo: %w", err)
	}
	checkPhoto, err := w.photos.LoadPhoto(ctx, job.CheckPhotoURL)
	if err != nil {
		return true, fmt.Errorf("biometricWorker load check photo: %w", err)
	}

	comparison, err := w.recognizer.CompareFaces(ctx, basePhoto, checkPhoto)
	if err != nil {
		return true, fmt.Errorf("biometricWorker compare faces: %w", err)
	}

	now := w.clock()
	status := empleados.ObservacionRechazada
	if comparison.Passed {
		status = empleados.ObservacionValidada
	}

	result := BiometricVerificationResult{
		AsistenciaID:       job.AsistenciaID,
		EmpleadoID:         job.EmpleadoID,
		EmpresaID:          job.EmpresaID,
		EstatusObservacion: status,
		SimilitudFacial:    comparison.Similarity,
		VerifiedAt:         now,
	}
	if err := w.repo.SaveBiometricResult(ctx, result); err != nil {
		return true, fmt.Errorf("biometricWorker save result: %w", err)
	}

	if w.publisher != nil {
		event := AsistenciaVerificadaEvent{
			AsistenciaID:       job.AsistenciaID,
			EmpleadoID:         job.EmpleadoID,
			EmpresaID:          job.EmpresaID,
			SimilitudFacial:    comparison.Similarity,
			EstatusObservacion: status,
			Timestamp:          now,
		}
		if err := w.publisher.PublishAsistenciaVerificada(ctx, event); err != nil {
			return true, fmt.Errorf("biometricWorker publish event: %w", err)
		}
	}

	return true, nil
}
