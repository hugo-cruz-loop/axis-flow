-- V10: Create bolsa_trabajo schema with trabajos, postulaciones, and evaluaciones tables.

CREATE SCHEMA IF NOT EXISTS bolsa_trabajo;

CREATE TABLE bolsa_trabajo.trabajos (
  id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  empresa_id      uuid        NOT NULL,
  titulo          varchar(150) NOT NULL,
  descripcion     text        NOT NULL,
  requisitos      jsonb       NULL,
  fecha_caducar   date        NOT NULL,
  estatus_vacante int         NOT NULL DEFAULT 1 CHECK (estatus_vacante IN (1, 2, 3)),
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_trabajos_empresa         ON bolsa_trabajo.trabajos (empresa_id);
CREATE INDEX idx_trabajos_estatus_caducar ON bolsa_trabajo.trabajos (estatus_vacante, fecha_caducar);

CREATE TABLE bolsa_trabajo.postulaciones (
  id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  trabajo_id      uuid        NOT NULL REFERENCES bolsa_trabajo.trabajos(id) ON DELETE CASCADE,
  nombre_completo varchar(150) NOT NULL,
  email           varchar(254) NOT NULL,
  telefono        varchar(30)  NULL,
  cv_url          text         NOT NULL,
  estatus         int          NOT NULL DEFAULT 1 CHECK (estatus IN (1, 2, 3, 4, 5)),
  created_at      timestamptz  NOT NULL DEFAULT now(),
  updated_at      timestamptz  NOT NULL DEFAULT now()
);
CREATE INDEX idx_postulaciones_trabajo ON bolsa_trabajo.postulaciones (trabajo_id);
CREATE INDEX idx_postulaciones_estatus  ON bolsa_trabajo.postulaciones (estatus);

CREATE TABLE bolsa_trabajo.evaluaciones (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  postulacion_id  uuid NOT NULL UNIQUE REFERENCES bolsa_trabajo.postulaciones(id) ON DELETE CASCADE,
  puntualidad     int  NOT NULL CHECK (puntualidad BETWEEN 1 AND 5),
  cortesia        int  NOT NULL CHECK (cortesia BETWEEN 1 AND 5),
  soft_skills     int  NOT NULL CHECK (soft_skills BETWEEN 1 AND 5),
  comentarios     text NULL,
  evaluator_id    uuid NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_evaluaciones_postulacion ON bolsa_trabajo.evaluaciones (postulacion_id);
