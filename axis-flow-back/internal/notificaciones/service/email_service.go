// Package service contains use-case implementations for the Notificaciones module.
package service

import (
	"context"
	"fmt"
	"log"
	"net/smtp"
	"strings"
	"time"

	"axis-flow-back/internal/notificaciones"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// EmailService — interface.
// ---------------------------------------------------------------------------

// EmailService defines transactional email operations.
type EmailService interface {
	// TriggerRecovery sends a password-recovery email to the given address.
	TriggerRecovery(ctx context.Context, email string) error
	// TriggerWelcome sends a welcome email to a newly created user.
	TriggerWelcome(ctx context.Context, userID uuid.UUID, email, name string) error
}

// ---------------------------------------------------------------------------
// noopEmailService — used when SMTPEnabled == false.
// ---------------------------------------------------------------------------

type noopEmailService struct{}

func (noopEmailService) TriggerRecovery(_ context.Context, _ string) error {
	log.Print("level=info msg=\"noop email skipped\" template=recovery_pass")
	return nil
}

func (noopEmailService) TriggerWelcome(_ context.Context, _ uuid.UUID, _, _ string) error {
	log.Print("level=info msg=\"noop email skipped\" template=welcome")
	return nil
}

// ---------------------------------------------------------------------------
// smtpEmailService — real SMTP implementation.
// ---------------------------------------------------------------------------

type smtpEmailService struct {
	cfg notificaciones.Config
}

func (s *smtpEmailService) TriggerRecovery(_ context.Context, email string) error {
	start := time.Now()
	addr := fmt.Sprintf("%s:%d", s.cfg.SMTPHost, s.cfg.SMTPPort)

	body := strings.Join([]string{
		"From: " + s.cfg.SMTPFromEmail,
		"To: " + email,
		"Subject: Recuperacion de contrasena",
		"",
		"Por favor usa el enlace adjunto para recuperar tu contrasena.",
	}, "\r\n")

	if err := smtp.SendMail(addr, s.auth(), s.cfg.SMTPFromEmail, []string{email}, []byte(body)); err != nil {
		return fmt.Errorf("email_service.TriggerRecovery: %w", err)
	}

	log.Printf("level=info msg=\"email dispatched\" template=recovery_pass recipient_email=%s duration_ms=%d",
		maskEmail(email), time.Since(start).Milliseconds())
	return nil
}

func (s *smtpEmailService) TriggerWelcome(_ context.Context, _ uuid.UUID, email, name string) error {
	start := time.Now()
	addr := fmt.Sprintf("%s:%d", s.cfg.SMTPHost, s.cfg.SMTPPort)

	body := strings.Join([]string{
		"From: " + s.cfg.SMTPFromEmail,
		"To: " + email,
		"Subject: Bienvenido",
		"",
		"Hola " + name + ", tu cuenta ha sido creada exitosamente.",
	}, "\r\n")

	if err := smtp.SendMail(addr, s.auth(), s.cfg.SMTPFromEmail, []string{email}, []byte(body)); err != nil {
		return fmt.Errorf("email_service.TriggerWelcome: %w", err)
	}

	log.Printf("level=info msg=\"email dispatched\" template=welcome recipient_email=%s duration_ms=%d",
		maskEmail(email), time.Since(start).Milliseconds())
	return nil
}

func (s *smtpEmailService) auth() smtp.Auth {
	if s.cfg.SMTPUsername == "" {
		return nil
	}
	return smtp.PlainAuth("", s.cfg.SMTPUsername, s.cfg.SMTPPassword, s.cfg.SMTPHost)
}

// maskEmail returns a masked representation of an email address.
// e.g. "user@example.com" → "u***@example.com"
// The raw address is NEVER logged.
func maskEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return "***@***"
	}
	local := parts[0]
	if len(local) == 0 {
		return "***@" + parts[1]
	}
	return string(local[0]) + "***@" + parts[1]
}

// ---------------------------------------------------------------------------
// Constructor.
// ---------------------------------------------------------------------------

// NewEmailService returns a noopEmailService when SMTPEnabled is false,
// otherwise a real smtpEmailService.
func NewEmailService(cfg notificaciones.Config) EmailService {
	if !cfg.SMTPEnabled {
		return noopEmailService{}
	}
	return &smtpEmailService{cfg: cfg}
}
