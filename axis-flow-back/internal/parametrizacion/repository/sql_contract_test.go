package repository_test

import (
	"strings"
	"testing"

	"axis-flow-back/internal/parametrizacion/repository"
)

func TestSQLContractsUseParametrizacionSchemaAndUpserts(t *testing.T) {
	contracts := map[string]string{
		"sistema":             repository.SistemaUpsertSQL,
		"evaluacion servicio": repository.EvaluacionServicioUpsertSQL,
		"evaluacion personal": repository.EvaluacionPersonalUpsertSQL,
		"dias umbral":         repository.DiasInactivosUmbralUpsertSQL,
	}
	for name, sql := range contracts {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(sql, "parametrizacion.") {
				t.Fatalf("%s query must target parametrizacion schema: %s", name, sql)
			}
			if !strings.Contains(strings.ToUpper(sql), "ON CONFLICT") {
				t.Fatalf("%s query must use upsert semantics: %s", name, sql)
			}
			if !strings.Contains(strings.ToUpper(sql), "RETURNING") {
				t.Fatalf("%s query must return persisted fields: %s", name, sql)
			}
		})
	}
}

func TestDeleteDiaUsesTenantScopedPredicate(t *testing.T) {
	upper := strings.ToUpper(repository.DiasInactivosDeleteSQL)
	if !strings.Contains(upper, "WHERE ID=$1 AND EMPRESA_ID=$2") {
		t.Fatalf("delete query must be scoped by id and empresa_id: %s", repository.DiasInactivosDeleteSQL)
	}
}
