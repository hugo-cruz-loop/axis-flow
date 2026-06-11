package parametrizacion_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrationContract(t *testing.T) {
	body, err := os.ReadFile(filepath.Clean("../../db/migrations/V14__create_parametrizacion_schema.sql"))
	if err != nil {
		t.Fatalf("read parametrizacion migration: %v", err)
	}
	sql := strings.ToLower(string(body))

	fragments := map[string][]string{
		"schema and tables": {
			"create schema if not exists parametrizacion",
			"create table if not exists parametrizacion.parametrizacion_sistema",
			"create table if not exists parametrizacion.parametrizacion_evaluacion_servicio",
			"create table if not exists parametrizacion.parametrizacion_evaluacion_personal",
			"create table if not exists parametrizacion.parametrizacion_dias_inactivos",
			"create table if not exists parametrizacion.parametrizacion_dias_inactivos_umbral",
		},
		"restrict foreign keys": {
			"constraint fk_evaluacion_servicio_empresa foreign key (empresa_id) references empresas.empresas_empresa (id) on delete restrict",
			"constraint fk_evaluacion_servicio_servicio foreign key (servicio_id) references catalogos.catalog_services (id) on delete restrict",
			"constraint fk_evaluacion_servicio_periodicidad foreign key (periodicidad_id) references catalogos.catalog_date_periodicities (id) on delete restrict",
			"constraint fk_evaluacion_personal_empresa foreign key (empresa_id) references empresas.empresas_empresa (id) on delete restrict",
			"constraint fk_evaluacion_personal_periodicidad foreign key (periodicidad_id) references catalogos.catalog_date_periodicities (id) on delete restrict",
			"constraint fk_dias_inactivos_empresa foreign key (empresa_id) references empresas.empresas_empresa (id) on delete restrict",
			"constraint fk_dias_inactivos_umbral_empresa foreign key (empresa_id) references empresas.empresas_empresa (id) on delete restrict",
		},
		"uniques checks indexes": {
			"constraint ck_clave_parametro_format check (clave_parametro ~ '^[a-z0-9_]+$')",
			"constraint uq_evaluacion_servicio_empresa_servicio unique (empresa_id, servicio_id)",
			"constraint uq_evaluacion_personal_empresa unique (empresa_id)",
			"constraint uq_dias_inactivos_empresa_fecha unique (empresa_id, fecha)",
			"umbral_dias integer not null default 5 check (umbral_dias >= 0)",
			"create index if not exists idx_eval_servicio_servicio_id",
			"create index if not exists idx_eval_servicio_periodicidad_id",
			"create index if not exists idx_eval_personal_periodicidad_id",
			"create index if not exists idx_dias_inactivos_empresa_fecha",
		},
		"timestamp triggers": {
			"create or replace function parametrizacion.fn_set_timestamp()",
			"create trigger trg_set_timestamp_sistema",
			"create trigger trg_set_timestamp_evaluacion_servicio",
			"create trigger trg_set_timestamp_evaluacion_personal",
			"create trigger trg_set_timestamp_dias_inactivos",
			"create trigger trg_set_timestamp_dias_inactivos_umbral",
		},
	}

	for group, groupFragments := range fragments {
		t.Run(group, func(t *testing.T) {
			for _, fragment := range groupFragments {
				if !strings.Contains(sql, fragment) {
					t.Fatalf("migration missing %q", fragment)
				}
			}
		})
	}
}
