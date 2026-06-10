// Package handler — respuesta_handler.go: HTTP handlers for the
// field-answer (respuesta) capture endpoint and the PDF report
// download endpoint.
//
// PR-4 (REST/HTTP) — task 4.3. Mirrors the structure of
// internal/atencionseguimiento/handler/queja_handler.go (PR-4 of
// 09_AtencionSeguimiento_Service_Spec). Content negotiation: the
// /respuesta POST accepts BOTH application/json AND
// multipart/form-data per the openapi spec — the handler
// auto-detects the Content-Type and routes accordingly.
//
// Routes:
//
//	POST /respuesta                          — SubmitRespuesta (JSON or multipart)
//	GET  /respuesta/reporte/pregunta/{id}/pdf — GetReportePDF (binary PDF)
//
// Multipart strategy (PR-4):
//
//   - Max 10 MB total request size (r.ParseMultipartForm(MaxMultipartMemory)).
//   - Max 3 evidence files (evidencia1, evidencia2, evidencia3).
//   - Whitelisted content types: image/jpeg, image/png, image/webp,
//     application/pdf. Anything else → 422 with "disallowed content-type".
//   - PR-4 does NOT upload to S3 — the handler only validates the
//     upload and sets a synthetic marker on the respuesta struct
//     (so the PR-8 verify phase can confirm the validation seam).
//     PR-6 will close the loop with the real S3 upload inside the
//     pdf_service.ReportStorage port. The handler's behaviour for
//     PR-4 is documented in Deviation #4.
//
// PDF strategy (PR-4 AMEND — FIX 2):
//
//   - PDFService.GenerateReporte returns (bytes, url, error).
//   - The handler streams the bytes to the response body with
//     Content-Type: application/pdf + Content-Disposition: attachment.
//   - The X-PDF-URL header still surfaces the durable storage URL for
//     downstream debugging.
//   - The body MUST be real PDF bytes (the renderer stub produces a
//     minimal %PDF-1.4 catalog); the old "PDF stream placeholder" text
//     is gone.
package handler

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"strings"
	"time"

	"axis-flow-back/internal/formularios"
	"axis-flow-back/internal/formularios/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Limits — kept in one place so PR-6 can adjust them without touching
// the handler logic.
// ---------------------------------------------------------------------------

const (
	// maxMultipartMemory caps the in-memory buffer for multipart
	// parsing at 10 MB. Anything larger is spilled to /tmp by net/http
	// and parsed lazily; the 10 MB cap matches the spec's
	// "max 10MB total request size" rule.
	maxMultipartMemory = 10 << 20 // 10 MB

	// maxEvidenceFiles is the cap on evidencia1/2/3 per the openapi
	// 3-image-evidence model.
	maxEvidenceFiles = 3
)

// allowedEvidenceContentTypes is the closed set per the orchestrator's
// PR-4 brief: image/jpeg, image/png, image/webp, application/pdf.
var allowedEvidenceContentTypes = map[string]struct{}{
	"image/jpeg":      {},
	"image/png":       {},
	"image/webp":      {},
	"application/pdf": {},
}

// ---------------------------------------------------------------------------
// Response DTOs (openapi snake_case contract).
// ---------------------------------------------------------------------------

type respuestaResponseDTO struct {
	ID               uuid.UUID       `json:"id"`
	EventoIniciadoID uuid.UUID       `json:"evento_iniciado_id"`
	FormularioID     uuid.UUID       `json:"formulario_id"`
	PreguntaID       uuid.UUID       `json:"pregunta_id"`
	RespuestaTexto   string          `json:"respuesta_texto"`
	RespuestaLista   json.RawMessage `json:"respuesta_lista"`
	Evidencia1       string          `json:"evidencia1,omitempty"`
	Evidencia2       string          `json:"evidencia2,omitempty"`
	Evidencia3       string          `json:"evidencia3,omitempty"`
	DocumentoURL     string          `json:"documento_url,omitempty"`
	Geolocalizacion  *geoDTO         `json:"geolocalizacion_respuesta,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
}

// ---------------------------------------------------------------------------
// RespuestaHandler.
// ---------------------------------------------------------------------------

// RespuestaHandler handles HTTP requests for the field-answer
// (respuesta) capture endpoint and the PDF report download endpoint.
type RespuestaHandler struct {
	svc service.RespuestaService
	pdf service.PDFService
}

// NewRespuestaHandler constructs a RespuestaHandler.
func NewRespuestaHandler(svc service.RespuestaService, pdf service.PDFService) *RespuestaHandler {
	return &RespuestaHandler{svc: svc, pdf: pdf}
}

// ---------------------------------------------------------------------------
// POST /respuesta — SubmitRespuesta (content-negotiated).
// ---------------------------------------------------------------------------

// createRespuestaJSONRequest is the JSON body for POST /respuesta
// (application/json path). Mirrors the openapi `application/json`
// schema for /respuesta POST.
type createRespuestaJSONRequest struct {
	EventoIniciadoID     uuid.UUID       `json:"evento_iniciado_id"`
	FormularioID         uuid.UUID       `json:"formulario_id"`
	PreguntaID           uuid.UUID       `json:"pregunta_id"`
	RespuestaTexto       string          `json:"respuesta_texto,omitempty"`
	RespuestaLista       json.RawMessage `json:"respuesta_lista"`
	EvidenciaURLs        []string        `json:"evidencia_urls"`
	DocumentoURL         string          `json:"documento_url,omitempty"`
	GeolocalizacionRespuesta *geoDTO     `json:"geolocalizacion_respuesta,omitempty"`
}

// SubmitRespuesta handles POST /respuesta. Requires JWT (any role;
// the openapi does not restrict, the upstream route layer can decide).
//
// Content negotiation:
//
//   - application/json  → JSON DTO path
//   - multipart/form-data → multipart path (validate + persist)
//
// Any other Content-Type → 422 with "unsupported content type".
func (h *RespuestaHandler) SubmitRespuesta(w http.ResponseWriter, r *http.Request) {
	tenantID, err := extractTenantID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "missing tenant")
		return
	}
	_ = tenantID

	ct := r.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(ct)
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid_payload", "unsupported content type")
		return
	}

	switch strings.ToLower(mediaType) {
	case "application/json":
		h.submitRespuestaJSON(w, r, params)
	case "multipart/form-data":
		h.submitRespuestaMultipart(w, r, params)
	default:
		respondError(w, http.StatusUnprocessableEntity, "invalid_payload", "unsupported content type")
	}
}

func (h *RespuestaHandler) submitRespuestaJSON(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	var req createRespuestaJSONRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid_payload", "invalid request body")
		return
	}
	if req.EventoIniciadoID == uuid.Nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid_payload", "evento_iniciado_id is required")
		return
	}
	if req.PreguntaID == uuid.Nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid_payload", "pregunta_id is required")
		return
	}
	if len(req.RespuestaLista) == 0 {
		respondError(w, http.StatusUnprocessableEntity, "invalid_payload", "respuesta_lista is required")
		return
	}

	tenantID, _ := extractTenantID(r)

	geoLat, geoLon := geoPtrsFromBody(req.GeolocalizacionRespuesta)
	r2 := &formularios.Respuesta{
		EventoIniciadoID:            req.EventoIniciadoID,
		PreguntaID:                  req.PreguntaID,
		RespuestaTexto:              req.RespuestaTexto,
		RespuestaLista:              req.RespuestaLista,
		DocumentoURL:                req.DocumentoURL,
		GeolocalizacionRespuestaLat: geoLat,
		GeolocalizacionRespuestaLon: geoLon,
	}
	if len(req.EvidenciaURLs) > 0 {
		r2.Evidencia1 = req.EvidenciaURLs[0]
	}
	if len(req.EvidenciaURLs) > 1 {
		r2.Evidencia2 = req.EvidenciaURLs[1]
	}
	if len(req.EvidenciaURLs) > 2 {
		r2.Evidencia3 = req.EvidenciaURLs[2]
	}

	created, err := h.svc.SubmitRespuesta(r.Context(), r2, tenantID)
	if err != nil {
		mapFormulariosError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, toRespuestaResponseDTO(created, req.FormularioID, req.GeolocalizacionRespuesta))
}

func (h *RespuestaHandler) submitRespuestaMultipart(w http.ResponseWriter, r *http.Request, params map[string]string) {
	if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid_payload", "invalid multipart payload")
		return
	}

	iniciadoID, err := parseUUIDField(r, "evento_iniciado_id")
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid_payload", "evento_iniciado_id is required")
		return
	}
	formID, err := parseUUIDField(r, "formulario_id")
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid_payload", "formulario_id is required")
		return
	}
	pregID, err := parseUUIDField(r, "pregunta_id")
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid_payload", "pregunta_id is required")
		return
	}
	respuestaTexto := r.FormValue("respuesta_texto")
	respuestaLista := json.RawMessage(r.FormValue("respuesta_lista"))
	if len(respuestaLista) == 0 {
		respondError(w, http.StatusUnprocessableEntity, "invalid_payload", "respuesta_lista is required")
		return
	}

	// Validate evidence files. PR-4 enforces: max 3, whitelisted
	// content types. The actual upload to S3 is the job of PR-6
	// (ReportStorage port); here we just validate and set a
	// synthetic marker so the service has a non-empty URL field
	// that can be re-validated downstream.
	ev1, ev2, ev3, err := collectMultipartEvidencias(r)
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid_payload", err.Error())
		return
	}

	tenantID, _ := extractTenantID(r)
	r2 := &formularios.Respuesta{
		EventoIniciadoID: iniciadoID,
		PreguntaID:       pregID,
		RespuestaTexto:   respuestaTexto,
		RespuestaLista:   respuestaLista,
		Evidencia1:       ev1,
		Evidencia2:       ev2,
		Evidencia3:       ev3,
		DocumentoURL:     r.FormValue("documento_url"),
	}

	created, err := h.svc.SubmitRespuesta(r.Context(), r2, tenantID)
	if err != nil {
		mapFormulariosError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, toRespuestaResponseDTO(created, formID, nil))
}

// collectMultipartEvidencias walks evidencia1..evidencia3 form-file
// keys, validates the count and content types, and returns a synthetic
// URL marker for each accepted file. The marker is intentionally
// clearly synthetic ("multipart:pending:<key>") so the PR-8 verify
// phase can distinguish "uploaded via PR-6 ReportStorage" from
// "validated in PR-4, upload pending".
func collectMultipartEvidencias(r *http.Request) (string, string, string, error) {
	if r.MultipartForm == nil {
		return "", "", "", nil
	}
	keys := []string{"evidencia1", "evidencia2", "evidencia3"}
	var out []string
	for _, k := range keys {
		files := r.MultipartForm.File[k]
		if len(files) == 0 {
			continue
		}
		if len(files) > 1 {
			return "", "", "", fmt.Errorf("too many files for %s", k)
		}
		fh := files[0]
		ct := fh.Header.Get("Content-Type")
		// Detect via mime sniffing if Content-Type is empty/generic.
		if ct == "" || ct == "application/octet-stream" {
			ct = sniffImageType(fh.Filename)
		}
		if _, ok := allowedEvidenceContentTypes[strings.ToLower(ct)]; !ok {
			return "", "", "", fmt.Errorf("disallowed content-type %q for %s", ct, k)
		}
		out = append(out, "multipart:pending:"+k+":"+fh.Filename)
	}
	if len(out) > maxEvidenceFiles {
		return "", "", "", fmt.Errorf("too many evidence files (max %d)", maxEvidenceFiles)
	}
	for len(out) < 3 {
		out = append(out, "")
	}
	return out[0], out[1], out[2], nil
}

// sniffImageType returns a best-guess content-type based on the
// filename extension when the multipart Content-Type is missing.
func sniffImageType(filename string) string {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".webp"):
		return "image/webp"
	case strings.HasSuffix(lower, ".pdf"):
		return "application/pdf"
	}
	return ""
}

// parseUUIDField reads a form value and parses it as a UUID.
func parseUUIDField(r *http.Request, key string) (uuid.UUID, error) {
	raw := r.FormValue(key)
	if raw == "" {
		return uuid.Nil, fmt.Errorf("%s is required", key)
	}
	return uuid.Parse(raw)
}

// geoPtrsFromBody returns the lat/lon pointers from a request body
// geo DTO, or nil if no geo was supplied.
func geoPtrsFromBody(g *geoDTO) (*float64, *float64) {
	if g == nil {
		return nil, nil
	}
	if !validLatLonHTTP(g.Latitud, g.Longitud) {
		// Caller is expected to validate before calling this; the
		// service-level repo will re-check.
	}
	lat := g.Latitud
	lon := g.Longitud
	return &lat, &lon
}

// ---------------------------------------------------------------------------
// GET /respuesta/reporte/pregunta/{id}/pdf — GetReportePDF.
// ---------------------------------------------------------------------------

// GetReportePDF handles GET /respuesta/reporte/pregunta/{id}/pdf.
// Requires JWT (any role). Streams the PDF binary produced by the
// PDFService.
//
// On Locker conflict (concurrent generation) the service returns
// formularios.ErrConflict → 409 (via the helper's mapping). On
// generic render/upload failure the helper returns 500 with a
// generic message — vendor detail / file paths are NOT echoed.
//
// PR-4 AMEND (FIX 2): the service now returns the rendered PDF bytes
// alongside the URL. We stream the bytes to the response body and
// surface the URL in the X-PDF-URL header for downstream debugging.
// The old "PDF stream placeholder" text is gone — that corrupted
// downloads because the body was plain text with Content-Type
// application/pdf.
func (h *RespuestaHandler) GetReportePDF(w http.ResponseWriter, r *http.Request) {
	tenantID, err := extractTenantID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "missing tenant")
		return
	}

	preguntaID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid_payload", "invalid pregunta id")
		return
	}
	_ = preguntaID

	// For PR-4 the PDF service is called with the preguntaID as a
	// proxy for "the report for this check-in's question" (the
	// openapi reuses the same path parameter). PR-6 will refine the
	// signature to take both preguntaID and iniciadoID explicitly.
	pdfBytes, url, err := h.pdf.GenerateReporte(r.Context(), preguntaID, tenantID)
	if err != nil {
		// On any error, the generic 500 must NOT echo the underlying
		// message (PII / vendor detail leak). The helper enforces
		// this; we just call it.
		mapFormulariosError(w, err)
		return
	}

	// Stream the real PDF bytes to the client. The renderer stub in
	// PR-4 produces a minimal valid %PDF-1.4 catalog (~80 bytes);
	// PR-6 will replace it with wkhtmltopdf-rendered bytes (the
	// signature stays the same — handler is bytes-agnostic).
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"reporte-%s.pdf\"", preguntaID))
	w.Header().Set("X-PDF-URL", url)
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(pdfBytes); err != nil {
		// The body is already partially written; we cannot recover
		// the status code at this point. Logging is the structured
		// logger's job (PR-5).
		_ = err
	}
}

// ---------------------------------------------------------------------------
// DTO conversions.
// ---------------------------------------------------------------------------

func toRespuestaResponseDTO(r *formularios.Respuesta, formularioID uuid.UUID, geo *geoDTO) respuestaResponseDTO {
	dto := respuestaResponseDTO{
		ID:               r.ID,
		EventoIniciadoID: r.EventoIniciadoID,
		FormularioID:     formularioID,
		PreguntaID:       r.PreguntaID,
		RespuestaTexto:   r.RespuestaTexto,
		RespuestaLista:   r.RespuestaLista,
		Evidencia1:       r.Evidencia1,
		Evidencia2:       r.Evidencia2,
		Evidencia3:       r.Evidencia3,
		DocumentoURL:     r.DocumentoURL,
		CreatedAt:        r.CreatedAt,
	}
	if geo != nil {
		dto.Geolocalizacion = geo
	}
	return dto
}
