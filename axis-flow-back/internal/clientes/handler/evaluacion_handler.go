package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"axis-flow-back/internal/clientes"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// EvaluacionServicer is the interface EvaluacionHandler depends on.
type EvaluacionServicer interface {
	CreateEvaluacion(ctx context.Context, e *clientes.EvaluacionServicio, empresaID int64) error
	ListEvaluaciones(ctx context.Context, clienteID uuid.UUID, empresaID int64) ([]clientes.EvaluacionServicio, error)
}

// EvaluacionHandler handles HTTP requests for evaluaciones.
type EvaluacionHandler struct {
	svc EvaluacionServicer
}

// NewEvaluacionHandler constructs an EvaluacionHandler.
func NewEvaluacionHandler(svc EvaluacionServicer) *EvaluacionHandler {
	return &EvaluacionHandler{svc: svc}
}

// ListEvaluaciones handles GET /cliente/{id}/evaluaciones
func (h *EvaluacionHandler) ListEvaluaciones(w http.ResponseWriter, r *http.Request) {
	clienteIDStr := chi.URLParam(r, "id")
	clienteID, err := uuid.Parse(clienteIDStr)
	if err != nil {
		writeError(w, "BAD_REQUEST", "invalid id", http.StatusBadRequest)
		return
	}

	empresaID, ok := parseEmpresaIDQuery(w, r)
	if !ok {
		return
	}

	list, err := h.svc.ListEvaluaciones(r.Context(), clienteID, empresaID)
	if err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

// CreateEvaluacion handles POST /cliente/{id}/evaluaciones
func (h *EvaluacionHandler) CreateEvaluacion(w http.ResponseWriter, r *http.Request) {
	clienteIDStr := chi.URLParam(r, "id")
	clienteID, err := uuid.Parse(clienteIDStr)
	if err != nil {
		writeError(w, "BAD_REQUEST", "invalid id", http.StatusBadRequest)
		return
	}

	empresaID, ok := parseEmpresaIDQuery(w, r)
	if !ok {
		return
	}

	var body struct {
		Puntuacion  int     `json:"puntuacion"`
		Comentarios *string `json:"comentarios"`
		DeUsuarioID string  `json:"de_usuario_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	if body.Puntuacion < 1 || body.Puntuacion > 5 {
		writeError(w, "BAD_REQUEST", "puntuacion must be between 1 and 5", http.StatusBadRequest)
		return
	}

	var deUsuarioID uuid.UUID
	if body.DeUsuarioID != "" {
		uid, parseErr := uuid.Parse(body.DeUsuarioID)
		if parseErr != nil {
			writeError(w, "BAD_REQUEST", "invalid de_usuario_id", http.StatusBadRequest)
			return
		}
		deUsuarioID = uid
	}

	e := &clientes.EvaluacionServicio{
		ClienteID:   clienteID,
		Puntuacion:  body.Puntuacion,
		Comentarios: body.Comentarios,
		DeUsuarioID: deUsuarioID,
	}
	if err := h.svc.CreateEvaluacion(r.Context(), e, empresaID); err != nil {
		writeClienteError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": e})
}
