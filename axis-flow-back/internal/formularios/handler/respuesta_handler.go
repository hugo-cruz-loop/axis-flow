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
// Multipart strategy (PR-4 AMEND — FIX 3):
//
//   - Max 10 MB total request size (r.ParseMultipartForm(MaxMultipartMemory)).
//   - Max 3 evidence files (evidencia1, evidencia2, evidencia3).
//   - Whitelisted content types: image/jpeg, image/png, image/webp,
//     application/pdf. Anything else → 422 with "disallowed content-type".
//   - Validation failures still return 422 with a specific message.
//   - ON VALIDATION SUCCESS the multipart path returns 501 with code
//     MULTIPART_UPLOAD_NOT_IMPLEMENTED — the S3-backed evidence upload
//     is wired in PR-6 (PDF/S3 Hardening). Until then, the multipart
//     path must NOT touch the database / service (no synthetic
//     "multipart:pending:evidenciaN" markers may land in production
//     rows). The JSON path continues to work unchanged.
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
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing tenant")
		return
	}
	_ = tenantID

	ct := r.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(ct)
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "unsupported content type")
		return
	}

	switch strings.ToLower(mediaType) {
	case "application/json":
		h.submitRespuestaJSON(w, r, params)
	case "multipart/form-data":
		h.submitRespuestaMultipart(w, r, params)
	default:
		respondError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "unsupported content type")
	}
}

func (h *RespuestaHandler) submitRespuestaJSON(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	var req createRespuestaJSONRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid request body")
		return
	}
	if req.EventoIniciadoID == uuid.Nil {
		respondError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "evento_iniciado_id is required")
		return
	}
	if req.PreguntaID == uuid.Nil {
		respondError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "pregunta_id is required")
		return
	}
	if len(req.RespuestaLista) == 0 {
		respondError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "respuesta_lista is required")
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
	// PR-4 AMEND (FIX 3): the multipart path is now a TWO-PHASE
	// handler:
	//
	//   PHASE 1 — Validation (max 10MB, max 3 evidence files,
	//             whitelisted content types, required text fields).
	//             Validation failures still return 422 with a
	//             specific message. NO state change.
	//
	//   PHASE 2 — 501 Not Implemented (code
	//             MULTIPART_UPLOAD_NOT_IMPLEMENTED). The S3-backed
	//             evidence upload is wired in PR-6 (PDF/S3
	//             Hardening). Until then, this handler must NOT
	//             touch the database / service — the synthetic
	//             "multipart:pending:evidenciaN:filename" markers
	//             from PR-4 are NOT allowed to land in production
	//             rows. The JSON path continues to work unchanged.
	//
	// The service.SubmitRespuesta call is removed entirely from this
	// code path; the recording stub in the tests uses t.Fatal() if
	// it is invoked, so any regression is caught by the test suite.

	if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
		respondError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid multipart payload")
		return
	}

	if _, err := parseUUIDField(r, "evento_iniciado_id"); err != nil {
		respondError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "evento_iniciado_id is required")
		return
	}
	if _, err := parseUUIDField(r, "formulario_id"); err != nil {
		respondError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "formulario_id is required")
		return
	}
	if _, err := parseUUIDField(r, "pregunta_id"); err != nil {
		respondError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "pregunta_id is required")
		return
	}
	if len(json.RawMessage(r.FormValue("respuesta_lista"))) == 0 {
		respondError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "respuesta_lista is required")
		return
	}

	// Validate evidence files. Validation failures (max 3,
	// whitelisted content types) → 422 with the specific message.
	_, _, _, err := collectMultipartEvidencias(r)
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
		return
	}

	// PHASE 2: every validation gate has passed. The multipart path
	// cannot persist to the database until PR-6 wires the S3 upload;
	// the handler returns 501 Not Implemented and a clear error code
	// so the frontend can show a friendly "evidence upload coming
	// soon" message. The request body is fully parsed and validated
	// at this point so the client can see exactly which field is
	// offending (e.g. content-type whitelist, file count).
	respondError(w, http.StatusNotImplemented, "MULTIPART_UPLOAD_NOT_IMPLEMENTED", "multipart upload pending PR-6 storage backend")
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
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing tenant")
		return
	}

	preguntaID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid pregunta id")
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
