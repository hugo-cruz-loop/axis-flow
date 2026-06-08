package unit_test

import (
	"testing"

	"axis-flow-back/internal/domain"

	"github.com/stretchr/testify/assert"
)

func TestValidateStatus_WithKnownStatuses_ReturnsNil(t *testing.T) {
	validStatuses := []domain.UserStatus{
		domain.StatusPendingActivation,
		domain.StatusActive,
		domain.StatusInactive,
		domain.StatusSuspended,
		domain.StatusLocked,
		domain.StatusDeleted,
	}
	for _, s := range validStatuses {
		t.Run(string(s), func(t *testing.T) {
			err := domain.ValidateStatus(s)
			assert.NoError(t, err)
		})
	}
}

func TestValidateStatus_WithUnknownStatus_ReturnsError(t *testing.T) {
	err := domain.ValidateStatus(domain.UserStatus("BOGUS"))
	assert.ErrorIs(t, err, domain.ErrInvalidStatus)
}

func TestCanLogin_WithActiveUser_ReturnsTrue(t *testing.T) {
	assert.True(t, domain.CanLogin(domain.StatusActive))
}

func TestCanLogin_WithNonActiveStatuses_ReturnsFalse(t *testing.T) {
	nonActive := []domain.UserStatus{
		domain.StatusPendingActivation,
		domain.StatusInactive,
		domain.StatusSuspended,
		domain.StatusLocked,
		domain.StatusDeleted,
	}
	for _, s := range nonActive {
		t.Run(string(s), func(t *testing.T) {
			assert.False(t, domain.CanLogin(s))
		})
	}
}
