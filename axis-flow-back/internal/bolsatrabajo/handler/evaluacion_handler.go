package handler

import (
	"context"
	"net/http"

	"axis-flow-back/internal/bolsatrabajo"
	"axis-flow-back/internal/bolsatrabajo/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// EvaluacionServicer is the interface EvaluacionHandler depends on.
type EvaluacionServicer interface {
	Create(ctx context.Context, e *bolsatrabajo.Evaluacion, empresaID uuid.UUID) (*bolsatrabajo.Evaluacion, error)
	GetByPostulacion(ctx context.Context, postulacionID, empresaID uuid.UUID) (*bolsatrabajo.Evaluacion, error)
}

// Compile-time check that service.EvaluacionService satisfies EvaluacionServicer.
var _ EvaluacionServicer = (service.EvaluacionService)(nil)

// EvaluacionHandler handles HTTP requests for post-interview evaluations.
type EvaluacionHandler struct {
	svc EvaluacionServicer
}

// NewEvaluacionHandler constructs an EvaluacionHandler.
func NewEvaluacionHandler(svc EvaluacionServicer) *EvaluacionHandler {
	return &EvaluacionHandler{svc: svc}
}

// Create handles POST /bolsa-trabajo/evaluacion — JWT required.
// Optionally reads Idempotency-Key header.
func (h *EvaluacionHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, err := extractTenantID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	userID, err := extractUserID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	// Optional idempotency key — read but not enforced at this layer (service/DB handles it).
	_ = r.Header.Get("Idempotency-Key")

	var body struct {
		PostulacionID string `json:"postulacion_id"`
		Puntualidad   int    `json:"puntualidad"`
		Cortesia      int    `json:"cortesia"`
		SoftSkills    int    `json:"soft_skills"`
		Comentarios   string `json:"comentarios"`
	}
	if err := jsonDecodeBody(r, &body); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	postulacionID, err := uuid.Parse(body.PostulacionID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid postulacionId")
		return
	}

	e := &bolsatrabajo.Evaluacion{
		PostulacionID: postulacionID,
		Puntualidad:   body.Puntualidad,
		Cortesia:      body.Cortesia,
		SoftSkills:    body.SoftSkills,
		Comentarios:   body.Comentarios,
		EvaluatorID:   userID,
	}

	created, err := h.svc.Create(r.Context(), e, tenantID)
	if err != nil {
		mapBolsaError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, created, newRequestID())
}

// GetByPostulacion handles GET /bolsa-trabajo/evaluacion/by-postulacion/{id} — JWT required.
func (h *EvaluacionHandler) GetByPostulacion(w http.ResponseWriter, r *http.Request) {
	tenantID, err := extractTenantID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	idStr := chi.URLParam(r, "id")
	postulacionID, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid postulacion id")
		return
	}

	eval, err := h.svc.GetByPostulacion(r.Context(), postulacionID, tenantID)
	if err != nil {
		mapBolsaError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, eval, newRequestID())
}
