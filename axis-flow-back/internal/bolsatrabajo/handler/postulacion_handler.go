package handler

import (
	"context"
	"io"
	"net/http"
	"time"

	"axis-flow-back/internal/bolsatrabajo"
	"axis-flow-back/internal/bolsatrabajo/service"
	"axis-flow-back/internal/bolsatrabajo/storage"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// PostulacionServicer is the interface PostulacionHandler depends on.
type PostulacionServicer interface {
	Apply(ctx context.Context, p *bolsatrabajo.Postulacion, cvReader io.Reader, cvSize int64, turnstileToken, remoteIP string) (*bolsatrabajo.Postulacion, error)
	GetStatsByTrabajo(ctx context.Context, trabajoID, empresaID uuid.UUID) (*bolsatrabajo.PipelineStats, error)
	UpdateEstatus(ctx context.Context, id, empresaID uuid.UUID, newEstatus int) (*bolsatrabajo.Postulacion, error)
}

// Compile-time check that service.PostulacionService satisfies PostulacionServicer.
var _ PostulacionServicer = (service.PostulacionService)(nil)

// postulacionHandlerOption is a functional option for PostulacionHandler.
type postulacionHandlerOption func(*PostulacionHandler)

// WithRateLimit configures the per-IP rate limiter on Apply.
func WithRateLimit(limit int, window time.Duration) postulacionHandlerOption {
	return func(h *PostulacionHandler) {
		h.limiter = NewIPRateLimiter(limit, window)
	}
}

// PostulacionHandler handles HTTP requests for job applications.
type PostulacionHandler struct {
	svc     PostulacionServicer
	limiter *IPRateLimiter
}

// NewPostulacionHandler constructs a PostulacionHandler.
// By default it applies a 10-req-per-minute rate limit on Apply.
func NewPostulacionHandler(svc PostulacionServicer, opts ...postulacionHandlerOption) *PostulacionHandler {
	h := &PostulacionHandler{
		svc:     svc,
		limiter: NewIPRateLimiter(10, time.Minute),
	}
	for _, o := range opts {
		o(h)
	}
	return h
}

// Apply handles POST /bolsa-trabajo/postulacion/apply — Public, rate-limited.
func (h *PostulacionHandler) Apply(w http.ResponseWriter, r *http.Request) {
	// Rate limit check
	ip := realIP(r)
	if !h.limiter.Allow(ip) {
		respondError(w, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "too many requests — please try again later")
		return
	}

	if err := r.ParseMultipartForm(storage.MaxCVSize + 1*1024*1024); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid multipart form")
		return
	}

	trabajoIDStr := r.FormValue("trabajoId")
	trabajoID, err := uuid.Parse(trabajoIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid trabajoId")
		return
	}

	nombreCompleto := r.FormValue("nombreCompleto")
	email := r.FormValue("email")
	telefono := r.FormValue("telefono")

	// Turnstile token — from header or form field
	turnstileToken := r.Header.Get("X-Turnstile-Token")
	if turnstileToken == "" {
		turnstileToken = r.FormValue("turnstileToken")
	}

	// Real IP — from X-Real-IP or RemoteAddr
	remoteIP := r.Header.Get("X-Real-IP")
	if remoteIP == "" {
		remoteIP = realIP(r)
	}

	// CV file
	file, fileHeader, err := r.FormFile("cv")
	if err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "cv file is required")
		return
	}
	defer file.Close()

	cvSize := fileHeader.Size
	if cvSize > storage.MaxCVSize {
		respondError(w, http.StatusUnprocessableEntity, "INVALID_FILE", "cv file exceeds 5 MB limit")
		return
	}

	p := &bolsatrabajo.Postulacion{
		TrabajoID:      trabajoID,
		NombreCompleto: nombreCompleto,
		Email:          email,
		Telefono:       telefono,
	}

	created, err := h.svc.Apply(r.Context(), p, file, cvSize, turnstileToken, remoteIP)
	if err != nil {
		mapBolsaError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, created, newRequestID())
}

// GetStats handles GET /bolsa-trabajo/postulacion/by-trabajo-stats/{id} — JWT required.
func (h *PostulacionHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	tenantID, err := extractTenantID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	idStr := chi.URLParam(r, "id")
	trabajoID, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid trabajo id")
		return
	}

	stats, err := h.svc.GetStatsByTrabajo(r.Context(), trabajoID, tenantID)
	if err != nil {
		mapBolsaError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, stats, newRequestID())
}

// UpdateEstatus handles PATCH /bolsa-trabajo/postulacion/status/{id} — JWT required.
func (h *PostulacionHandler) UpdateEstatus(w http.ResponseWriter, r *http.Request) {
	tenantID, err := extractTenantID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid postulacion id")
		return
	}

	var body struct {
		Estatus int `json:"estatus"`
	}
	if err := jsonDecodeBody(r, &body); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	updated, err := h.svc.UpdateEstatus(r.Context(), id, tenantID, body.Estatus)
	if err != nil {
		mapBolsaError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, updated, newRequestID())
}
