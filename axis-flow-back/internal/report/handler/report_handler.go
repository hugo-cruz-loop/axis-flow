// Package handler provides HTTP handlers for the ReportBro report service.
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"axis-flow-back/internal/middleware"
	report "axis-flow-back/internal/report"
	"axis-flow-back/internal/report/signing"
)

// ReportCacheIface is the cache port consumed by ReportHandler.
// Declared here so tests can substitute fakes without importing the cache package.
type ReportCacheIface interface {
	Store(ctx context.Context, key string, data []byte, ttl time.Duration) error
	Fetch(ctx context.Context, key string) ([]byte, error)
}

// PDFGeneratorIface is the PDF generation port consumed by ReportHandler.
type PDFGeneratorIface interface {
	Generate(ctx context.Context, req report.ReportRequest) ([]byte, error)
}

// XLSXGeneratorIface is the XLSX generation port consumed by ReportHandler.
type XLSXGeneratorIface interface {
	Generate(ctx context.Context, req report.ReportRequest) ([]byte, error)
}

// ReportHandler handles report generation and signed binary download.
type ReportHandler struct {
	cache      ReportCacheIface
	pdfGen     PDFGeneratorIface
	xlsxGen    XLSXGeneratorIface
	signingKey string
	ttl        time.Duration
}

// NewReportHandler constructs a ReportHandler. signingKey MUST NOT be logged anywhere.
func NewReportHandler(
	c ReportCacheIface,
	pdf PDFGeneratorIface,
	xlsx XLSXGeneratorIface,
	signingKey string,
	ttl time.Duration,
) *ReportHandler {
	return &ReportHandler{
		cache:      c,
		pdfGen:     pdf,
		xlsxGen:    xlsx,
		signingKey: signingKey,
		ttl:        ttl,
	}
}

// runReportRequest is the parsed POST body.
type runReportRequest struct {
	Filters      report.ReportFilters `json:"filters"`
	OutputFormat string               `json:"outputFormat"`
}

// runReportResponse is the POST 200 body.
type runReportResponse struct {
	Data struct {
		Key         string `json:"key"`
		DownloadURL string `json:"download_url"`
		ExpiresAt   string `json:"expires_at"`
	} `json:"data"`
}

// writeError writes a JSON error body.
func writeError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

// RunReport handles POST /api/v1/report/run/{type}.
// JWT middleware must already have populated the context with userID and tenantID.
func (h *ReportHandler) RunReport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract identity from JWT context (set by middleware.JWTAuth).
	userID, _ := ctx.Value(middleware.ContextKeyUserID).(string)
	empresaID, _ := ctx.Value(middleware.ContextKeyTenantID).(string)

	reportType := chi.URLParam(r, "type")

	var req runReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY")
		return
	}

	// Validate output format.
	var outputFmt report.OutputFormat
	switch req.OutputFormat {
	case string(report.FormatPDF):
		outputFmt = report.FormatPDF
	case string(report.FormatXLSX):
		outputFmt = report.FormatXLSX
	default:
		writeError(w, http.StatusBadRequest, "INVALID_FORMAT")
		return
	}

	domainReq := report.ReportRequest{
		Type:         reportType,
		Filters:      req.Filters,
		OutputFormat: outputFmt,
		UserID:       userID,
		EmpresaID:    empresaID,
	}

	// Dispatch to correct generator.
	var (
		data []byte
		err  error
	)
	switch outputFmt {
	case report.FormatPDF:
		data, err = h.pdfGen.Generate(ctx, domainReq)
	case report.FormatXLSX:
		data, err = h.xlsxGen.Generate(ctx, domainReq)
	}
	if err != nil {
		status, code := mapReportError(err)
		writeError(w, status, code)
		return
	}

	// Store in cache under a fresh UUID key.
	key := uuid.NewString()
	if err := h.cache.Store(ctx, key, data, h.ttl); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}

	// Build signed download URL.
	expires := strconv.FormatInt(time.Now().Add(h.ttl).Unix(), 10)
	sig := signing.Sign(key, expires, userID, h.signingKey)
	downloadURL := fmt.Sprintf("/api/v1/report/run/%s?expires=%s&uid=%s&sig=%s", key, expires, userID, sig)
	expiresAt := time.Now().Add(h.ttl).Format(time.RFC3339)

	var resp runReportResponse
	resp.Data.Key = key
	resp.Data.DownloadURL = downloadURL
	resp.Data.ExpiresAt = expiresAt

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// pdfMagic is the PDF binary magic bytes.
var pdfMagic = []byte("%PDF")

// DownloadReport handles GET /api/v1/report/run/{key}.
// No JWT required — the HMAC signature is the auth mechanism.
func (h *ReportHandler) DownloadReport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	key := chi.URLParam(r, "key")

	q := r.URL.Query()
	expires := q.Get("expires")
	uid := q.Get("uid")
	sig := q.Get("sig")

	// Expiry check (before HMAC to short-circuit early).
	expiresInt, err := strconv.ParseInt(expires, 10, 64)
	if err != nil || time.Now().Unix() > expiresInt {
		writeError(w, http.StatusForbidden, "FORBIDDEN")
		return
	}

	// Signature verification — uses constant-time comparison inside signing.Verify.
	if !signing.Verify(key, expires, uid, sig, h.signingKey) {
		writeError(w, http.StatusForbidden, "FORBIDDEN")
		return
	}

	data, err := h.cache.Fetch(ctx, key)
	if err != nil {
		status, code := mapReportError(err)
		writeError(w, status, code)
		return
	}

	// Detect Content-Type from magic bytes (never trust user-supplied header).
	var contentType, filename string
	if bytes.HasPrefix(data, pdfMagic) {
		contentType = "application/pdf"
		filename = "report.pdf"
	} else {
		contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		filename = "report.xlsx"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
