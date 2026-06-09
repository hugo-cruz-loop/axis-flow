package handler

import (
	"context"
	"fmt"
	"net/http"
)

// CertificateServicer is the interface CertificateHandler depends on.
type CertificateServicer interface {
	GenerateCertificate(ctx context.Context, empleadoID, examenID int64) ([]byte, string, error)
}

// CertificateHandler handles HTTP requests for certificate downloads.
type CertificateHandler struct {
	svc CertificateServicer
}

// NewCertificateHandler constructs a CertificateHandler.
func NewCertificateHandler(svc CertificateServicer) *CertificateHandler {
	return &CertificateHandler{svc: svc}
}

// DownloadCertificate handles GET /certificado/{examen_id}/download?empleado_id=
func (h *CertificateHandler) DownloadCertificate(w http.ResponseWriter, r *http.Request) {
	_, ok := tenantFromCtx(w, r)
	if !ok {
		return
	}
	examenID, ok := parsePathInt64(w, r, "examen_id")
	if !ok {
		return
	}
	empleadoID, ok := parseQueryInt64(w, r, "empleado_id")
	if !ok {
		return
	}

	pdfBytes, filename, err := h.svc.GenerateCertificate(r.Context(), empleadoID, examenID)
	if err != nil {
		writeCursosError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdfBytes)
}
