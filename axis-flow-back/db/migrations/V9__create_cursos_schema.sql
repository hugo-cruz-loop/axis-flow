-- V9: Create cursos schema with 12 tables, indexes, and progress trigger.

CREATE SCHEMA IF NOT EXISTS cursos;

-- 1. Categoria
CREATE TABLE cursos.cursos_categoria (
    id          bigserial PRIMARY KEY,
    nombre      text      NOT NULL,
    descripcion text,
    empresa_id  uuid      NOT NULL REFERENCES empresas(id)
);

-- 2. Modulo
CREATE TABLE cursos.cursos_modulo (
    id         bigserial PRIMARY KEY,
    nombre     text NOT NULL,
    empresa_id uuid NOT NULL REFERENCES empresas(id)
);

-- 3. Curso
CREATE TABLE cursos.cursos_curso (
    id                 bigserial PRIMARY KEY,
    titulo             text      NOT NULL,
    descripcion        text,
    empresa_id         uuid      NOT NULL REFERENCES empresas(id),
    modulo_id          bigint    REFERENCES cursos.cursos_modulo(id),
    categoria_id       bigint    NOT NULL REFERENCES cursos.cursos_categoria(id),
    estatus            smallint  NOT NULL DEFAULT 1,
    imagen_url         text,
    duracion_minutos   int       DEFAULT 0,
    prerequisito_id    bigint    REFERENCES cursos.cursos_curso(id),
    created_at         timestamptz DEFAULT now(),
    deleted_at         timestamptz
);

-- 4. Unidad
CREATE TABLE cursos.cursos_unidad (
    id       bigserial PRIMARY KEY,
    curso_id bigint NOT NULL REFERENCES cursos.cursos_curso(id) ON DELETE CASCADE,
    titulo   text   NOT NULL,
    orden    int    NOT NULL DEFAULT 0
);

-- 5. Leccion
CREATE TABLE cursos.cursos_leccion (
    id           bigserial PRIMARY KEY,
    unidad_id    bigint   NOT NULL REFERENCES cursos.cursos_unidad(id) ON DELETE CASCADE,
    titulo       text     NOT NULL,
    tipo         smallint NOT NULL,
    contenido_url text,
    orden        int      NOT NULL DEFAULT 0
);

-- 6. Notas
CREATE TABLE cursos.cursos_notas (
    id          bigserial PRIMARY KEY,
    leccion_id  bigint NOT NULL REFERENCES cursos.cursos_leccion(id) ON DELETE CASCADE,
    empleado_id bigint NOT NULL,
    contenido   text   NOT NULL,
    updated_at  timestamptz DEFAULT now(),
    UNIQUE (leccion_id, empleado_id)
);

-- 7. Enrollment
CREATE TABLE cursos.cursos_enroll_curso (
    id                 bigserial PRIMARY KEY,
    curso_id           bigint          NOT NULL REFERENCES cursos.cursos_curso(id),
    empleado_id        bigint          NOT NULL,
    empresa_id         uuid            NOT NULL,
    avance_porcentaje  numeric(5,2)    DEFAULT 0,
    estatus            smallint        NOT NULL DEFAULT 2,
    enrolled_at        timestamptz     DEFAULT now(),
    completed_at       timestamptz,
    UNIQUE (curso_id, empleado_id)
);

-- 8. Avance leccion
CREATE TABLE cursos.cursos_avance_leccion (
    empleado_id  bigint      NOT NULL,
    leccion_id   bigint      NOT NULL REFERENCES cursos.cursos_leccion(id) ON DELETE CASCADE,
    completado_at timestamptz DEFAULT now(),
    PRIMARY KEY (empleado_id, leccion_id)
);

-- 9. Examen
CREATE TABLE cursos.cursos_examen (
    id               bigserial    PRIMARY KEY,
    curso_id         bigint       NOT NULL UNIQUE REFERENCES cursos.cursos_curso(id) ON DELETE CASCADE,
    titulo           text         NOT NULL,
    note_min         numeric(4,2) NOT NULL DEFAULT 60,
    num_intentos     smallint     NOT NULL DEFAULT 3,
    tiempo_limite_min int
);

-- 10. Examen pregunta
CREATE TABLE cursos.cursos_examen_pregunta (
    id         bigserial PRIMARY KEY,
    examen_id  bigint    NOT NULL REFERENCES cursos.cursos_examen(id) ON DELETE CASCADE,
    enunciado  text      NOT NULL,
    orden      int       DEFAULT 0
);

-- 11. Examen opcion
CREATE TABLE cursos.cursos_examen_opcion (
    id          bigserial PRIMARY KEY,
    pregunta_id bigint  NOT NULL REFERENCES cursos.cursos_examen_pregunta(id) ON DELETE CASCADE,
    texto       text    NOT NULL,
    es_correcta boolean NOT NULL DEFAULT false
);

-- 12. Resultados examen
CREATE TABLE cursos.cursos_resultados_examen (
    id           bigserial    PRIMARY KEY,
    examen_id    bigint       NOT NULL REFERENCES cursos.cursos_examen(id),
    empleado_id  bigint       NOT NULL,
    calificacion numeric(5,2),
    aprobado     boolean,
    intento      int          NOT NULL DEFAULT 1,
    created_at   timestamptz  DEFAULT now()
);

-- Indexes
CREATE INDEX ON cursos.cursos_curso(empresa_id);
CREATE INDEX ON cursos.cursos_curso(modulo_id);
CREATE INDEX ON cursos.cursos_enroll_curso(empleado_id);
CREATE INDEX ON cursos.cursos_enroll_curso(empresa_id);
CREATE INDEX ON cursos.cursos_avance_leccion(empleado_id);
CREATE INDEX ON cursos.cursos_examen_pregunta(examen_id);
CREATE INDEX ON cursos.cursos_resultados_examen(examen_id, empleado_id);
CREATE INDEX ON cursos.cursos_notas(empleado_id);
CREATE INDEX ON cursos.cursos_leccion(unidad_id);

-- Progress trigger function
CREATE OR REPLACE FUNCTION cursos.fn_update_enrollment_progress()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
    v_curso_id        bigint;
    v_total_lecciones int;
    v_completadas     int;
    v_porcentaje      numeric(5,2);
BEGIN
    SELECT u.curso_id INTO v_curso_id
    FROM cursos.cursos_unidad u
    JOIN cursos.cursos_leccion l ON l.unidad_id = u.id
    WHERE l.id = COALESCE(NEW.leccion_id, OLD.leccion_id)
    LIMIT 1;

    SELECT COUNT(*) INTO v_total_lecciones
    FROM cursos.cursos_leccion l
    JOIN cursos.cursos_unidad u ON u.id = l.unidad_id
    WHERE u.curso_id = v_curso_id;

    SELECT COUNT(*) INTO v_completadas
    FROM cursos.cursos_avance_leccion al
    JOIN cursos.cursos_leccion l ON l.id = al.leccion_id
    JOIN cursos.cursos_unidad u ON u.id = l.unidad_id
    WHERE u.curso_id = v_curso_id
      AND al.empleado_id = COALESCE(NEW.empleado_id, OLD.empleado_id);

    IF v_total_lecciones > 0 THEN
        v_porcentaje := (v_completadas::numeric / v_total_lecciones) * 100;
    ELSE
        v_porcentaje := 0;
    END IF;

    UPDATE cursos.cursos_enroll_curso
    SET avance_porcentaje = v_porcentaje,
        estatus = CASE WHEN v_porcentaje >= 100 THEN 1 ELSE 2 END,
        completed_at = CASE WHEN v_porcentaje >= 100 THEN now() ELSE NULL END
    WHERE curso_id = v_curso_id
      AND empleado_id = COALESCE(NEW.empleado_id, OLD.empleado_id);

    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_update_enrollment_progress
AFTER INSERT OR UPDATE OR DELETE ON cursos.cursos_avance_leccion
FOR EACH ROW EXECUTE FUNCTION cursos.fn_update_enrollment_progress();
