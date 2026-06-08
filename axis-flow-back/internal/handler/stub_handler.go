package handler

import "net/http"

// NotImplemented returns 501 for endpoints defined in spec but not yet implemented (TBD).
func NotImplemented(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	_, _ = w.Write([]byte(`{"error":"not implemented"}`))
}
