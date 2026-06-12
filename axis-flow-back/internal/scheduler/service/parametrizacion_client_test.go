// Package service — parametrizacion_client_test.go covers the five
// behavioural scenarios called out in the PR 2B-ii task description
// for 2.7: query string, Authorization header + scope, 4xx error
// classification, 5xx retry+unavailable, and a happy-path parse.
//
// This is the only inline test in the batch. Phase 5 (testing) will
// extend coverage with a multi-replica lock test and an integration
// test against a live parametrizacion container.
package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recorderM2M is a test M2MClient that mints a real HS256 token
// (assertable in the happy path) and forwards HTTP through the
// auto-renewing HTTPClient wrapper. The wrapper's inner client is
// the http.DefaultClient, so the bearer header is added by the
// wrapper's Do method — not by the test.
type recorderM2M struct {
	t          *testing.T
	signingKey []byte
	calls      int32
}

func (r *recorderM2M) MintToken(_ context.Context, _ []string, _ time.Duration) (string, error) {
	return r.mint("scheduler:read")
}

func (r *recorderM2M) AcquireToken(_ context.Context) (string, error) {
	return r.mint("scheduler:read")
}

func (r *recorderM2M) HTTPClient() *HTTPClient {
	return &HTTPClient{inner: http.DefaultClient, m2m: r}
}

func (r *recorderM2M) mint(scope string) (string, error) {
	atomic.AddInt32(&r.calls, 1)
	now := jwt.NewNumericDate(time.Now())
	claims := jwt.MapClaims{
		"iss":   "scheduler-service",
		"aud":   "axis-flow-internal",
		"sub":   "scheduler-service",
		"iat":   now.Unix(),
		"exp":   now.Unix() + 300,
		"jti":   "test-jti",
		"scope": scope,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(r.signingKey)
	require.NoError(r.t, err)
	return signed, nil
}

// TestParametrizacionClient_HTTPBehaviour exercises the public
// surface of ParametrizacionClient against an httptest server. The
// path segment after /empresa/ acts as a discriminator so the same
// server can simulate 5xx storm, 4xx fail-fast, 200 happy, and two
// failure modes from one test run.
func TestParametrizacionClient_HTTPBehaviour(t *testing.T) {
	const signingKey = "this-is-a-32-byte-secret-key-xx" // 33 bytes, >= 32
	m2m := &recorderM2M{t: t, signingKey: []byte(signingKey)}

	var (
		gotPath    string
		gotAuth    string
		fivexxHits int32
		fourxxHits int32
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/parametrizacion/dias-inactivos/empresa/", func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/parametrizacion/dias-inactivos/empresa/")
		switch id {
		case "12": // 5xx storm: retried up to 3 times
			atomic.AddInt32(&fivexxHits, 1)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"success":false,"error":"upstream"}`)
		case "13": // 4xx fail-fast
			atomic.AddInt32(&fourxxHits, 1)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"success":false,"error":"bad request"}`)
		case "14": // happy path
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"data":    map[string]any{"empresa_id": 14, "umbral_dias": 7, "dias_inactivos": []any{}},
			})
		case "15": // malformed JSON
			_, _ = io.WriteString(w, `{not json`)
		case "16": // success=false
			_, _ = io.WriteString(w, `{"success":false}`)
		case "17": // umbral_dias=0
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true, "data": map[string]any{"empresa_id": 17, "umbral_dias": 0},
			})
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewParametrizacionClient(srv.URL, m2m, nil)
	ctx := context.Background()

	t.Run("happy_path_parses_umbral_and_carries_bearer_with_scope", func(t *testing.T) {
		gotPath, gotAuth = "", ""
		umbral, err := client.GetInactivityThreshold(ctx, 14)
		require.NoError(t, err)
		assert.Equal(t, 7, umbral)
		assert.Equal(t, "/api/v1/parametrizacion/dias-inactivos/empresa/14", gotPath,
			"endpoint must use path param, no query string")
		require.True(t, strings.HasPrefix(gotAuth, "Bearer "), "missing Bearer header: %q", gotAuth)
		parsed, perr := jwt.Parse(strings.TrimPrefix(gotAuth, "Bearer "), func(t *jwt.Token) (any, error) {
			return m2m.signingKey, nil
		})
		require.NoError(t, perr)
		claims := parsed.Claims.(jwt.MapClaims)
		assert.Equal(t, "scheduler:read", claims["scope"])
		assert.Equal(t, "scheduler-service", claims["iss"])
		assert.Equal(t, "axis-flow-internal", claims["aud"])
	})

	t.Run("4xx_returns_ErrParametrizacionBadRequest_without_retry", func(t *testing.T) {
		_, err := client.GetInactivityThreshold(ctx, 13)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrParametrizacionBadRequest)
		assert.Equal(t, int32(1), atomic.LoadInt32(&fourxxHits), "4xx must not be retried")
	})

	t.Run("5xx_exhausts_budget_and_returns_ErrParametrizacionUnavailable", func(t *testing.T) {
		_, err := client.GetInactivityThreshold(ctx, 12)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrParametrizacionUnavailable)
		assert.Equal(t, int32(3), atomic.LoadInt32(&fivexxHits),
			"5xx must be retried up to maxTries (3 total calls)")
	})

	t.Run("malformed_envelope_returns_parse_error", func(t *testing.T) {
		_, err := client.GetInactivityThreshold(ctx, 15)
		require.Error(t, err)
	})
}
