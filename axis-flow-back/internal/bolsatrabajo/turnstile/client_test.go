package turnstile_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"axis-flow-back/internal/bolsatrabajo"
	"axis-flow-back/internal/bolsatrabajo/turnstile"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMockTurnstile_AlwaysPasses verifies the mock always returns nil.
func TestMockTurnstile_AlwaysPasses(t *testing.T) {
	client := turnstile.New(false, "", "")
	err := client.Verify(context.Background(), "any-token", "1.2.3.4")
	require.NoError(t, err)
}

// TestHTTPTurnstile_SuccessTrue_ReturnsNil verifies a success:true response maps to nil.
func TestHTTPTurnstile_SuccessTrue_ReturnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	}))
	defer srv.Close()

	client := turnstile.New(true, "secret", srv.URL)
	err := client.Verify(context.Background(), "valid-token", "1.2.3.4")
	require.NoError(t, err)
}

// TestHTTPTurnstile_SuccessFalse_ReturnsCaptchaFail verifies a success:false maps to ErrCaptchaFail.
func TestHTTPTurnstile_SuccessFalse_ReturnsCaptchaFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false})
	}))
	defer srv.Close()

	client := turnstile.New(true, "secret", srv.URL)
	err := client.Verify(context.Background(), "bad-token", "1.2.3.4")
	assert.ErrorIs(t, err, bolsatrabajo.ErrCaptchaFail)
}
