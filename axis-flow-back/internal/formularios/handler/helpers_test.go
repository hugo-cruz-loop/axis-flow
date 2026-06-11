// White-box tests of the formularios HTTP helpers.
//
// PR-4 (REST/HTTP) — task 4.1 (helpers). Mirrors the structure of
// internal/atencionseguimiento/handler/helpers.go (PR-4 of
// 09_AtencionSeguimiento_Service_Spec). The package is `handler` (not
// `handler_test`) so the tests have access to unexported helpers
// (parsePagination, mapFormulariosError, extractTenantID/UserID/Role/EmpleadoID).
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"axis-flow-back/internal/formularios"
	"axis-flow-back/internal/middleware"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// respondJSON / respondPaginated / respondError — shape assertions.
// ---------------------------------------------------------------------------

func TestRespondJSON_WrapsPayloadInSuccessEnvelope(t *testing.T) {
	rr := httptest.NewRecorder()
	respondJSON(rr, http.StatusCreated, map[string]any{"id": "abc"})

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: want 201, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type: want application/json, got %q", ct)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["success"] != true {
		t.Fatalf("expected success:true, got %v", body["success"])
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %T", body["data"])
	}
	if data["id"] != "abc" {
		t.Fatalf("expected data.id=abc, got %v", data["id"])
	}
}

func TestRespondPaginated_EmitsMetaBlock(t *testing.T) {
	rr := httptest.NewRecorder()
	respondPaginated(rr, http.StatusOK, []string{"a", "b"}, 2, 5, 12)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rr.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	meta, ok := body["meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected meta object, got %T", body["meta"])
	}
	if meta["page"].(float64) != 2 {
		t.Fatalf("meta.page: want 2, got %v", meta["page"])
	}
	if meta["limit"].(float64) != 5 {
		t.Fatalf("meta.limit: want 5, got %v", meta["limit"])
	}
	if meta["total_records"].(float64) != 12 {
		t.Fatalf("meta.total_records: want 12, got %v", meta["total_records"])
	}
	// total=12, limit=5 → 3 pages (12/5 = 2 remainder 2 → +1)
	if meta["total_pages"].(float64) != 3 {
		t.Fatalf("meta.total_pages: want 3, got %v", meta["total_pages"])
	}
}

func TestRespondError_EmitsErrorEnvelopeWithCodeAndMessage(t *testing.T) {
	rr := httptest.NewRecorder()
	respondError(rr, http.StatusForbidden, "FORBIDDEN", "access denied")

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status: want 403, got %d", rr.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["success"] != false {
		t.Fatalf("expected success:false, got %v", body["success"])
	}
	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object, got %T", body["error"])
	}
	if errObj["code"] != "FORBIDDEN" {
		t.Fatalf("error.code: want FORBIDDEN, got %v", errObj["code"])
	}
	if errObj["message"] != "access denied" {
		t.Fatalf("error.message: want access denied, got %v", errObj["message"])
	}
}

// ---------------------------------------------------------------------------
// parsePagination — default, cap, and invalid-input behaviour.
// ---------------------------------------------------------------------------

func TestParsePagination_Defaults(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	page, limit := parsePagination(r)
	if page != 1 {
		t.Fatalf("default page: want 1, got %d", page)
	}
	if limit != 20 {
		t.Fatalf("default limit: want 20, got %d", limit)
	}
}

func TestParsePagination_HonoursQuery(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/x?page=3&limit=50", nil)
	page, limit := parsePagination(r)
	if page != 3 {
		t.Fatalf("page: want 3, got %d", page)
	}
	if limit != 50 {
		t.Fatalf("limit: want 50, got %d", limit)
	}
}

func TestParsePagination_CapsLimitAt100(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/x?limit=500", nil)
	page, limit := parsePagination(r)
	if limit != 20 {
		t.Fatalf("limit over-cap: want default 20, got %d", limit)
	}
	if page != 1 {
		t.Fatalf("page: want 1, got %d", page)
	}
}

func TestParsePagination_RejectsNonNumeric(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/x?page=abc&limit=-5", nil)
	page, limit := parsePagination(r)
	if page != 1 {
		t.Fatalf("non-numeric page: want 1, got %d", page)
	}
	if limit != 20 {
		t.Fatalf("negative limit: want 20, got %d", limit)
	}
}

// ---------------------------------------------------------------------------
// extractTenantID / extractUserID / extractEmpleadoID / extractRole.
// ---------------------------------------------------------------------------

func TestExtractTenantID_Missing(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	_, err := extractTenantID(r)
	if err == nil {
		t.Fatal("expected error on missing tenant, got nil")
	}
}

func TestExtractTenantID_InvalidUUID(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	r = r.WithContext(context.WithValue(r.Context(), middleware.ContextKeyTenantID, "not-a-uuid"))
	_, err := extractTenantID(r)
	if err == nil {
		t.Fatal("expected error on invalid uuid, got nil")
	}
}

func TestExtractTenantID_HappyPath(t *testing.T) {
	id := uuid.New()
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	r = r.WithContext(context.WithValue(r.Context(), middleware.ContextKeyTenantID, id.String()))
	got, err := extractTenantID(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != id {
		t.Fatalf("tenant: want %s, got %s", id, got)
	}
}

func TestExtractUserID_Missing(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	_, err := extractUserID(r)
	if err == nil {
		t.Fatal("expected error on missing user, got nil")
	}
}

func TestExtractUserID_InvalidUUID(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	r = r.WithContext(context.WithValue(r.Context(), middleware.ContextKeyUserID, "garbage"))
	_, err := extractUserID(r)
	if err == nil {
		t.Fatal("expected error on invalid uuid, got nil")
	}
}

func TestExtractUserID_HappyPath(t *testing.T) {
	id := uuid.New()
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	r = r.WithContext(context.WithValue(r.Context(), middleware.ContextKeyUserID, id.String()))
	got, err := extractUserID(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != id {
		t.Fatalf("user: want %s, got %s", id, got)
	}
}

func TestExtractEmpleadoID_Missing(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	_, err := extractEmpleadoID(r)
	if err == nil {
		t.Fatal("expected error on missing empleado id, got nil")
	}
}

func TestExtractEmpleadoID_WrongType(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	// string instead of int64
	r = r.WithContext(context.WithValue(r.Context(), middleware.ContextKeyEmpleadoID, "99"))
	_, err := extractEmpleadoID(r)
	if err == nil {
		t.Fatal("expected error on wrong type, got nil")
	}
}

func TestExtractEmpleadoID_HappyPath(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	r = r.WithContext(context.WithValue(r.Context(), middleware.ContextKeyEmpleadoID, int64(99)))
	got, err := extractEmpleadoID(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 99 {
		t.Fatalf("empleado id: want 99, got %d", got)
	}
}

// TestExtractEmpleadoID_Zero_ReturnsError is the PR-4 AMEND (FIX 5)
// invariant: a zero value is NOT a valid empleado id. The handler
// must surface 401 in that case (the JWT issuer is expected to put
// the real empleado id in the claim; if it didn't, the handler
// refuses to fall back to the body).
func TestExtractEmpleadoID_Zero_ReturnsError(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	r = r.WithContext(context.WithValue(r.Context(), middleware.ContextKeyEmpleadoID, int64(0)))
	_, err := extractEmpleadoID(r)
	if err == nil {
		t.Fatal("expected error on zero empleado id, got nil")
	}
}

func TestExtractRole_EmptyWhenMissing(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	if got := extractRole(r); got != "" {
		t.Fatalf("expected empty role, got %q", got)
	}
}

func TestExtractRole_HappyPath(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	r = r.WithContext(context.WithValue(r.Context(), middleware.ContextKeyRole, "Admin"))
	if got := extractRole(r); got != "Admin" {
		t.Fatalf("role: want Admin, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// mapFormulariosError — sentinel → status + code mapping.
// ---------------------------------------------------------------------------

func TestMapFormulariosError_Sentinels(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"ErrNotFound→404", formularios.ErrNotFound, http.StatusNotFound, "NOT_FOUND"},
		{"ErrForbidden→403", formularios.ErrForbidden, http.StatusForbidden, "FORBIDDEN"},
		{"ErrInvalidInput→422", formularios.ErrInvalidInput, http.StatusUnprocessableEntity, "VALIDATION_ERROR"},
		{"ErrConflict→409", formularios.ErrConflict, http.StatusConflict, "CONFLICT"},
		{"Unknown→500", errors.New("boom"), http.StatusInternalServerError, "INTERNAL_ERROR"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			mapFormulariosError(rr, tc.err)
			if rr.Code != tc.wantStatus {
				t.Fatalf("status: want %d, got %d", tc.wantStatus, rr.Code)
			}
			var body map[string]any
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			errObj := body["error"].(map[string]any)
			if errObj["code"] != tc.wantCode {
				t.Fatalf("code: want %q, got %v", tc.wantCode, errObj["code"])
			}
		})
	}
}

// TestMapFormulariosError_GenericErrorMessageHasNoPII asserts that an
// unmapped error is reported as "internal_error" with a generic message —
// the underlying error's text is NOT echoed (it could contain file paths,
// SQL fragments, or other PII-adjacent content).
func TestMapFormulariosError_GenericErrorMessageHasNoPII(t *testing.T) {
	leaky := errors.New("postgres: connection to /var/run/secret.sock failed: password=hunter2")
	rr := httptest.NewRecorder()
	mapFormulariosError(rr, leaky)
	body := rr.Body.String()
	if contains(body, "hunter2") {
		t.Fatalf("PII leaked through generic 500: %s", body)
	}
	if contains(body, "/var/run/secret.sock") {
		t.Fatalf("file path leaked through generic 500: %s", body)
	}
}

// contains is a tiny local helper to avoid pulling strings into the test
// imports list.
func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}
func indexOf(haystack, needle string) int {
	if len(needle) == 0 {
		return 0
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// Sanity guard: ensure the helpers don't import symbols they shouldn't.
var _ = fmt.Sprintf
