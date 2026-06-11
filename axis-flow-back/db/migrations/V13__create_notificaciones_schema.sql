-- V13: create notificaciones schema with push token log and atencion alert tables.

CREATE SCHEMA IF NOT EXISTS notificaciones;

CREATE TABLE notificaciones.notificacionenviada (
  id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     uuid        NOT NULL,
  token       text        NOT NULL,
  noti_id     varchar(150) NULL,
  device_type varchar(20) NOT NULL CHECK (device_type IN ('WEB', 'ANDROID', 'IOS')),
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_noti_enviada_user
  ON notificaciones.notificacionenviada (user_id);

CREATE TABLE notificaciones.notificacionatencionserviciocliente (
  id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    uuid        NOT NULL,
  ticket_id  int         NULL,
  queja_id   int         NULL,
  mensaje    text        NOT NULL,
  estatus    int         NOT NULL DEFAULT 1 CHECK (estatus IN (1, 2)),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_atencion_user_estatus
  ON notificaciones.notificacionatencionserviciocliente (user_id, estatus);
