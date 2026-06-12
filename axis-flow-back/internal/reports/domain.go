// Package reports provides the domain types, sentinel errors, role constants,
// and PII helpers for the Reports service. It owns the reports schema and
// exposes contracts consumed by the repository, service, and handler layers.
package reports

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// ---------------------------------------------------------------------------
// Role constants — mirror the JWT role claim values used across axis-flow.
// ---------------------------------------------------------------------------

const (
	// RoleSupervisor is the field-supervisor role.
	RoleSupervisor = "supervisor"
	// RoleHR is the human-resources role; receives un-masked employee names.
	RoleHR = "rh"
	// RoleAdmin is the platform-administrator role.
	RoleAdmin = "admin"
	// RoleClient is the external-client role.
	RoleClient = "cliente"
)

// ---------------------------------------------------------------------------
// Sentinel errors.
// ---------------------------------------------------------------------------

var (
	// ErrNotFound is returned when a requested resource does not exist.
	ErrNotFound = errors.New("reports: not found")
	// ErrForbidden is returned when the caller lacks permission for the
	// requested resource (IDOR, tenant mismatch, etc.).
	ErrForbidden = errors.New("reports: forbidden")
	// ErrRateLimitExceeded is returned when the geocoding rate limit is
	// reached and the caller must back off.
	ErrRateLimitExceeded = errors.New("reports: geocoding rate limit exceeded")
	// ErrGeocodingUnavailable is returned when neither the cache nor the
	// GCP Geocoding API can resolve the coordinates.
	ErrGeocodingUnavailable = errors.New("reports: geocoding unavailable")
)

// ---------------------------------------------------------------------------
// Domain structs.
// ---------------------------------------------------------------------------

// Evidencia represents a single activity evidence record produced by a field
// employee. Evidencias holds the S3 URLs of photo attachments.
type Evidencia struct {
	AsignacionID         string
	ActividadID          string
	EmpleadoNombre       string
	ActividadDescripcion string
	FechaEjecucion       string
	Latitud              float64
	Longitud             float64
	Evidencias           []string
}

// AsistenciaRecord represents a single clock-in/clock-out attendance record.
// DelayMinutes is positive when the employee was late.
type AsistenciaRecord struct {
	ID             string
	EmpleadoNombre string
	EmpleadoCodigo string
	ClockIn        string
	ClockOut       string
	Status         string
	DelayMinutes   int
	Latitud        float64
	Longitud       float64
}

// GraficaEvidenciaStat holds weekly aggregated evidence compliance data used
// for chart rendering. Compliant is the count that met the evidence threshold.
type GraficaEvidenciaStat struct {
	Week      string
	Total     int
	Compliant int
}

// IncidenteCount pairs an incident status label with its occurrence count.
type IncidenteCount struct {
	Status string
	Count  int
}

// DireccionCache is the persistence representation of a reverse-geocoding
// cache entry stored in reports.reports_direcciones.
type DireccionCache struct {
	Latitud   float64
	Longitud  float64
	Direccion string
	CreatedAt string
	UpdatedAt string
}

// GeocodingResult is the output produced by the geocoding service. Cached is
// true when the result was served from Redis or the PostgreSQL cache table.
type GeocodingResult struct {
	Latitud   float64
	Longitud  float64
	Direccion string
	Cached    bool
}

// ---------------------------------------------------------------------------
// PII helpers.
// ---------------------------------------------------------------------------

// MaskEmpleadoName reduces a full employee name to "First_initial. LastName"
// to protect PII in contexts where only non-HR roles are allowed full names.
//
// Rules:
//   - Empty string → ""
//   - Single word  → "X." (initial + period)
//   - Multi-word   → "X. LastWord" (first rune of first word + last word)
func MaskEmpleadoName(fullName string) string {
	if fullName == "" {
		return ""
	}

	words := strings.Fields(fullName)
	if len(words) == 0 {
		return ""
	}

	// Extract the first rune of the first word as the initial.
	firstRune, _ := utf8.DecodeRuneInString(words[0])
	initial := string(firstRune)

	if len(words) == 1 {
		return initial + "."
	}

	lastName := words[len(words)-1]
	return initial + ". " + lastName
}
