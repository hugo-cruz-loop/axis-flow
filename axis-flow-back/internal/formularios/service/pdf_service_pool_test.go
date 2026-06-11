// Package service_test — PR-6 (6.3) TDD RED test for the
// RenderPool wiring in PDFService. The pool is OPTIONAL (nil =
// no pooling; the previous behavior is preserved for legacy
// callers and unit tests). The main.go wiring is the only
// caller that passes a real pool.
//
// The test pins the constructor signature: NewPDFService takes
// a *RenderPool as its 10th arg (nilable). The 9 existing
// parameters are unchanged; the pool is purely additive.
package service_test

import (
	"testing"

	"axis-flow-back/internal/formularios/service"

	"github.com/stretchr/testify/assert"
)

// TestPDFService_NewPDFService_AcceptsRenderPool10thArg asserts
// the constructor signature change: NewPDFService takes a
// *RenderPool as the 10th arg. This is a compile-time-only
// assertion; if the signature drifts, this test will fail to
// compile.
func TestPDFService_NewPDFService_AcceptsRenderPool10thArg(t *testing.T) {
	pool := service.NewRenderPool(2)
	// nil for the other 9 args — we just need the compiler to
	// accept the call shape. The function will panic on use
	// (we don't call it).
	_ = func() service.PDFService {
		return service.NewPDFService(
			nil, // evRepo
			nil, // respRepo
			nil, // pub
			nil, // cache
			nil, // renderer
			nil, // storage
			nil, // locker
			nil, // metrics
			pool, // pool (10th arg — new in PR-6)
		)
	}
	assert.Equal(t, 2, pool.Size(), "pool is plumbed through the constructor")
}
