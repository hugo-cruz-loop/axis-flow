package main

import (
	"axis-flow-back/internal/bolsatrabajo/events"
	bthandler "axis-flow-back/internal/bolsatrabajo/handler"
)

// newBolsaTrabajoModuleForTest builds a bolsaTrabajoModule backed by stub
// implementations so that route registration tests do not require a real DB or Redis.
func newBolsaTrabajoModuleForTest() *bolsaTrabajoModule {
	trabajoH := bthandler.NewTrabajoHandler(&stubTrabajoSvc{})
	postulacionH := bthandler.NewPostulacionHandler(&stubPostulacionSvc{})
	evaluacionH := bthandler.NewEvaluacionHandler(&stubEvaluacionSvc{})
	consumer := events.NewEmpresaDeBajaConsumer(nil, &stubTrabajoSvc{})

	return &bolsaTrabajoModule{
		trabajoH:     trabajoH,
		postulacionH: postulacionH,
		evaluacionH:  evaluacionH,
		Consumer:     consumer,
	}
}
