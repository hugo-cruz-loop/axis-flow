package service_test

import (
	"context"
	"testing"

	"axis-flow-back/internal/notificaciones"
	"axis-flow-back/internal/notificaciones/service"

	"github.com/google/uuid"
)

func TestNewEmailService_NoopWhenSMTPDisabled(t *testing.T) {
	cfg := notificaciones.Config{SMTPEnabled: false}
	svc := service.NewEmailService(cfg)
	if svc == nil {
		t.Fatal("expected non-nil EmailService")
	}
}

func TestNoopEmailService_TriggerRecovery(t *testing.T) {
	cfg := notificaciones.Config{SMTPEnabled: false}
	svc := service.NewEmailService(cfg)

	err := svc.TriggerRecovery(context.Background(), "test@example.com")
	if err != nil {
		t.Fatalf("TriggerRecovery: expected nil error, got %v", err)
	}
}

func TestNoopEmailService_TriggerWelcome(t *testing.T) {
	cfg := notificaciones.Config{SMTPEnabled: false}
	svc := service.NewEmailService(cfg)

	err := svc.TriggerWelcome(context.Background(), uuid.New(), "user@example.com", "Alice")
	if err != nil {
		t.Fatalf("TriggerWelcome: expected nil error, got %v", err)
	}
}

func TestNewEmailService_SMTPImpl_WhenEnabled(t *testing.T) {
	cfg := notificaciones.Config{
		SMTPEnabled:   true,
		SMTPHost:      "localhost",
		SMTPPort:      25,
		SMTPFromEmail: "no-reply@example.com",
	}
	svc := service.NewEmailService(cfg)
	if svc == nil {
		t.Fatal("expected non-nil EmailService for SMTP mode")
	}
	// We only assert the constructor returns non-nil.
	// Real SMTP dials are integration-tested separately.
}
