CREATE SCHEMA IF NOT EXISTS clientes;

-- 1. Tabla Principal de Clientes
CREATE TABLE clientes.clientes_cliente (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id bigint NOT NULL,
    representante_id uuid NOT NULL,
    nombre_comercial varchar(255) NOT NULL,
    razon_social varchar(255),
    fecha_inicio_contrato date,
    estatus integer DEFAULT 2 NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT fk_clientes_cliente_empresa FOREIGN KEY (empresa_id) REFERENCES empresas.empresas_empresa(id) ON DELETE RESTRICT,
    CONSTRAINT fk_clientes_cliente_representante FOREIGN KEY (representante_id) REFERENCES users.identity_users(id) ON DELETE RESTRICT,
    CONSTRAINT ck_clientes_cliente_estatus CHECK (estatus IN (1, 2))
);

-- 2. Datos Fiscales 1:1
CREATE TABLE clientes.clientes_factura (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    cliente_id uuid UNIQUE NOT NULL,
    rfc varchar(512) NOT NULL, -- stored AES-256 encrypted
    razon_social varchar(255) NOT NULL,
    domicilio_fiscal text NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT fk_clientes_factura_cliente FOREIGN KEY (cliente_id) REFERENCES clientes.clientes_cliente(id) ON DELETE CASCADE
);

-- 3. Presupuesto 1:1
CREATE TABLE clientes.clientes_presupuesto (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    cliente_id uuid UNIQUE NOT NULL,
    personal_requerido integer CHECK (personal_requerido >= 0),
    material_estimado text,
    costo_mensual numeric(12,2) CHECK (costo_mensual >= 0.00),
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT fk_clientes_presupuesto_cliente FOREIGN KEY (cliente_id) REFERENCES clientes.clientes_cliente(id) ON DELETE CASCADE
);

-- 4. Calendario Laboral 1:1 (JSONB)
CREATE TABLE clientes.clientes_calendariolaboral (
    cliente_id uuid PRIMARY KEY,
    semana_laboral jsonb NOT NULL DEFAULT '{"monday":true,"tuesday":true,"wednesday":true,"thursday":true,"friday":true,"saturday":false,"sunday":false}'::jsonb,
    dias_inhabiles jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT fk_clientes_calendariolaboral_cliente FOREIGN KEY (cliente_id) REFERENCES clientes.clientes_cliente(id) ON DELETE CASCADE
);

-- 5. Localidades / Sucursales 1:N
-- NOTE: supervisor_id column exists but FK to hr.employees is intentionally omitted.
-- The hr module does not exist yet. FK will be added in a future migration once hr is implemented.
CREATE TABLE clientes.clientes_localidad (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    cliente_id uuid NOT NULL,
    nombre varchar(255) NOT NULL,
    direccion text NOT NULL,
    supervisor_id uuid NOT NULL,
    tipo_localidad_id bigint NOT NULL,
    latitud numeric(9,6) CHECK (latitud BETWEEN -90.000000 AND 90.000000),
    longitud numeric(9,6) CHECK (longitud BETWEEN -180.000000 AND 180.000000),
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT fk_clientes_localidad_cliente FOREIGN KEY (cliente_id) REFERENCES clientes.clientes_cliente(id) ON DELETE CASCADE,
    CONSTRAINT fk_clientes_localidad_tipo FOREIGN KEY (tipo_localidad_id) REFERENCES catalogos.catalog_locality_types(id) ON DELETE RESTRICT
);

-- 6. Servicios por Localidad M:N
CREATE TABLE clientes.clientes_servicioslocalidad (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    localidad_id uuid NOT NULL,
    servicio_id bigint NOT NULL,
    status_activo boolean DEFAULT true NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT fk_clientes_servicioslocalidad_localidad FOREIGN KEY (localidad_id) REFERENCES clientes.clientes_localidad(id) ON DELETE CASCADE,
    CONSTRAINT fk_clientes_servicioslocalidad_servicio FOREIGN KEY (servicio_id) REFERENCES catalogos.catalog_services(id) ON DELETE RESTRICT,
    CONSTRAINT uq_localidad_servicio UNIQUE (localidad_id, servicio_id)
);

-- 7. Horarios por Localidad
CREATE TABLE clientes.clientes_horario (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    localidad_id uuid NOT NULL,
    hora_entrada time NOT NULL,
    hora_salida time NOT NULL,
    hora_comida_inicio time,
    hora_comida_fin time,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT fk_clientes_horario_localidad FOREIGN KEY (localidad_id) REFERENCES clientes.clientes_localidad(id) ON DELETE CASCADE,
    CONSTRAINT ck_clientes_horario_salida CHECK (hora_entrada < hora_salida)
);

-- 8. Herramientas por Localidad
CREATE TABLE clientes.clientes_herramienta (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    localidad_id uuid NOT NULL,
    nombre varchar(255) NOT NULL,
    cantidad integer DEFAULT 1 NOT NULL CHECK (cantidad > 0),
    especificaciones text,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT fk_clientes_herramienta_localidad FOREIGN KEY (localidad_id) REFERENCES clientes.clientes_localidad(id) ON DELETE CASCADE
);

-- 9. Actividades por Localidad
CREATE TABLE clientes.clientes_actividad (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    localidad_id uuid NOT NULL,
    descripcion varchar(255) NOT NULL,
    frecuencia varchar(50) NOT NULL,
    orden integer DEFAULT 1 NOT NULL CHECK (orden > 0),
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT fk_clientes_actividad_localidad FOREIGN KEY (localidad_id) REFERENCES clientes.clientes_localidad(id) ON DELETE CASCADE
);

-- 10. Evaluaciones de Servicio
CREATE TABLE clientes.clientes_evaluacionservicio (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    cliente_id uuid NOT NULL,
    puntuacion integer NOT NULL CONSTRAINT ck_evaluacion_puntuacion CHECK (puntuacion BETWEEN 1 AND 5),
    comentarios text,
    de_usuario_id uuid NOT NULL,
    fecha date DEFAULT CURRENT_DATE NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT fk_clientes_evaluacion_cliente FOREIGN KEY (cliente_id) REFERENCES clientes.clientes_cliente(id) ON DELETE CASCADE,
    CONSTRAINT fk_clientes_evaluacion_usuario FOREIGN KEY (de_usuario_id) REFERENCES users.identity_users(id) ON DELETE RESTRICT
);

-- Quality Gate trigger: prevents activating a cliente (estatus=1) unless
-- Datos Fiscales, Presupuesto, and Calendario Laboral are all configured.
CREATE OR REPLACE FUNCTION clientes.fn_check_client_activation_integrity()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.estatus = 1 THEN
        IF NOT EXISTS (SELECT 1 FROM clientes.clientes_factura WHERE cliente_id = NEW.id) THEN
            RAISE EXCEPTION 'Quality Gate: Falta configurar Datos Fiscales del Cliente' USING ERRCODE = '23514';
        END IF;
        IF NOT EXISTS (SELECT 1 FROM clientes.clientes_presupuesto WHERE cliente_id = NEW.id) THEN
            RAISE EXCEPTION 'Quality Gate: Falta configurar Presupuesto del Cliente' USING ERRCODE = '23514';
        END IF;
        IF NOT EXISTS (SELECT 1 FROM clientes.clientes_calendariolaboral WHERE cliente_id = NEW.id) THEN
            RAISE EXCEPTION 'Quality Gate: Falta configurar Calendario Laboral' USING ERRCODE = '23514';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_client_activation_integrity
    BEFORE INSERT OR UPDATE ON clientes.clientes_cliente
    FOR EACH ROW
    WHEN (NEW.estatus = 1)
    EXECUTE FUNCTION clientes.fn_check_client_activation_integrity();
