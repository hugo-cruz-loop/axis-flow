CREATE SCHEMA IF NOT EXISTS empleados;

-- 1. Tabla Maestra de Empleados
CREATE TABLE empleados.empleados_empleado (
    num_empleado bigint GENERATED ALWAYS AS IDENTITY,
    id_empleado varchar(50) NOT NULL,
    usuario_id uuid NOT NULL,
    empresa_id bigint NOT NULL,
    nombre varchar(70) NOT NULL,
    apellido_paterno varchar(70) NOT NULL,
    apellido_materno varchar(70),
    status integer NOT NULL DEFAULT 2, -- 1=Activo, 2=Incompleto, 4=Baja
    created_at timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT pk_empleados_empleado PRIMARY KEY (num_empleado),
    CONSTRAINT uq_empleados_empleado_usuario UNIQUE (usuario_id),
    CONSTRAINT uq_empleados_id_empleado_empresa UNIQUE (empresa_id, id_empleado),
    CONSTRAINT fk_empleados_empleado_usuario FOREIGN KEY (usuario_id) REFERENCES users.identity_users(id) ON DELETE RESTRICT,
    CONSTRAINT fk_empleados_empleado_empresa FOREIGN KEY (empresa_id) REFERENCES empresas.empresas_empresa(id) ON DELETE RESTRICT,
    CONSTRAINT ck_empleados_empleado_status CHECK (status IN (1, 2, 4))
);

-- 2. Datos de Ubicación y Domicilio (1:1)
CREATE TABLE empleados.empleados_ubicacion (
    empleado_id bigint NOT NULL,
    curp varchar(18) NOT NULL,
    nss varchar(11) NOT NULL,
    calle varchar(255) NOT NULL,
    numero_exterior varchar(50) NOT NULL,
    numero_interior varchar(50),
    colonia varchar(150) NOT NULL,
    codigo_postal varchar(5) NOT NULL,
    ciudad_id bigint NOT NULL,
    estado_id bigint NOT NULL,
    pais_id bigint NOT NULL,
    created_at timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT pk_empleados_ubicacion PRIMARY KEY (empleado_id),
    CONSTRAINT uq_empleados_ubicacion_curp UNIQUE (curp),
    CONSTRAINT uq_empleados_ubicacion_nss UNIQUE (nss),
    CONSTRAINT fk_empleados_ubicacion_empleado FOREIGN KEY (empleado_id) REFERENCES empleados.empleados_empleado(num_empleado) ON DELETE CASCADE
);

-- 3. Contactos Adicionales y Beneficiarios (1:1)
CREATE TABLE empleados.empleados_adicionales (
    empleado_id bigint NOT NULL,
    contacto_emergencia_nombre varchar(255) NOT NULL,
    contacto_emergencia_telefono varchar(20) NOT NULL,
    contacto_emergencia_parentesco varchar(100) NOT NULL,
    beneficiarios jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_at timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT pk_empleados_adicionales PRIMARY KEY (empleado_id),
    CONSTRAINT fk_empleados_adicionales_empleado FOREIGN KEY (empleado_id) REFERENCES empleados.empleados_empleado(num_empleado) ON DELETE CASCADE
);

-- 4. Expediente de Documentación Digital (1:1)
CREATE TABLE empleados.empleados_documentos (
    empleado_id bigint NOT NULL,
    acta_url varchar(1024),
    ine_url varchar(1024),
    comprobante_domicilio_url varchar(1024),
    curp_pdf_url varchar(1024),
    nss_pdf_url varchar(1024),
    contrato_url varchar(1024),
    estatus_validacion integer DEFAULT 1 NOT NULL,
    created_at timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT pk_empleados_documentos PRIMARY KEY (empleado_id),
    CONSTRAINT ck_empleados_documentos_estatus CHECK (estatus_validacion IN (1, 2, 3)),
    CONSTRAINT fk_empleados_documentos_empleado FOREIGN KEY (empleado_id) REFERENCES empleados.empleados_empleado(num_empleado) ON DELETE CASCADE
);

-- 5. Tabla de Asistencias Diarias (IMMUTABLE)
CREATE TABLE empleados.empleados_asistencias (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empleado_id bigint NOT NULL,
    geolocalizacion varchar(100) NOT NULL,
    estatus_rango integer NOT NULL,
    foto_entrada_url varchar(1024) NOT NULL,
    estatus_observacion_entrada integer DEFAULT 1 NOT NULL,
    similitud_facial numeric(5,2),
    tipo_registro varchar(30) NOT NULL DEFAULT 'ENTRADA_LABORAL',
    fecha date DEFAULT CURRENT_DATE NOT NULL,
    hora_entrada time NOT NULL,
    hora_salida time,
    created_at timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_empleados_asistencias_rango CHECK (estatus_rango IN (1, 2)),
    CONSTRAINT ck_empleados_asistencias_observacion CHECK (estatus_observacion_entrada IN (1, 2, 3)),
    CONSTRAINT ck_empleados_asistencias_tipo CHECK (tipo_registro IN ('ENTRADA_LABORAL', 'SALIDA_LABORAL', 'ENTRADA_COMIDA', 'SALIDA_COMIDA')),
    CONSTRAINT fk_empleados_asistencias_empleado FOREIGN KEY (empleado_id) REFERENCES empleados.empleados_empleado(num_empleado) ON DELETE RESTRICT
);

-- 6. Asistencia en Comida (1:1)
CREATE TABLE empleados.empleados_asistencias_comida (
    asistencia_id uuid NOT NULL,
    hora_salida time NOT NULL,
    hora_entrada time,
    created_at timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT pk_empleados_asistencias_comida PRIMARY KEY (asistencia_id),
    CONSTRAINT fk_empleados_asistencias_comida_padre FOREIGN KEY (asistencia_id) REFERENCES empleados.empleados_asistencias(id) ON DELETE CASCADE
);

-- 7. Inasistencias / Incidencias
CREATE TABLE empleados.empleados_inasistencia (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empleado_id bigint NOT NULL,
    tipo_incidencia varchar(50) NOT NULL,
    fecha_inicio date NOT NULL,
    fecha_fin date NOT NULL,
    justificante_url varchar(1024),
    aprobado boolean DEFAULT false NOT NULL,
    observaciones varchar(250),
    resolved_by uuid,
    resolved_at timestamptz,
    created_at timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT fk_empleados_inasistencia_empleado FOREIGN KEY (empleado_id) REFERENCES empleados.empleados_empleado(num_empleado) ON DELETE RESTRICT,
    CONSTRAINT fk_empleados_inasistencia_resolver FOREIGN KEY (resolved_by) REFERENCES users.identity_users(id) ON DELETE RESTRICT
);

-- 8. Fotos base biométricas
CREATE TABLE empleados.empleados_fotologin (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empleado_id bigint NOT NULL,
    foto_base_url varchar(1024) NOT NULL,
    created_at timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT fk_empleados_fotologin_empleado FOREIGN KEY (empleado_id) REFERENCES empleados.empleados_empleado(num_empleado) ON DELETE CASCADE
);

-- 9. Dispositivos Móviles Autorizados
CREATE TABLE empleados.empleados_user_devices (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empleado_id bigint NOT NULL,
    device_uuid varchar(100) NOT NULL,
    device_model varchar(100),
    os_version varchar(50),
    fcm_token varchar(255) NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT uq_empleado_device UNIQUE (empleado_id, device_uuid),
    CONSTRAINT fk_empleados_devices_empleado FOREIGN KEY (empleado_id) REFERENCES empleados.empleados_empleado(num_empleado) ON DELETE CASCADE
);

-- Indexes
CREATE INDEX idx_empleados_empleado_empresa ON empleados.empleados_empleado(empresa_id);
CREATE INDEX idx_empleados_empleado_usuario ON empleados.empleados_empleado(usuario_id);
CREATE INDEX idx_empleados_asistencias_empleado_fecha ON empleados.empleados_asistencias(empleado_id, fecha);
CREATE INDEX idx_empleados_asistencias_pendiente ON empleados.empleados_asistencias(estatus_observacion_entrada) WHERE estatus_observacion_entrada = 1;
CREATE INDEX idx_empleados_inasistencia_empleado ON empleados.empleados_inasistencia(empleado_id);
CREATE INDEX idx_empleados_devices_lookup ON empleados.empleados_user_devices(empleado_id, device_uuid);

-- Immutability trigger
CREATE OR REPLACE FUNCTION empleados.prevent_asistencias_tampering()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'Eliminación prohibida: Los registros de asistencia son inmutables para cumplir con auditorías laborales.';
    ELSIF TG_OP = 'UPDATE' THEN
        IF OLD.empleado_id <> NEW.empleado_id OR
           OLD.geolocalizacion <> NEW.geolocalizacion OR
           OLD.estatus_rango <> NEW.estatus_rango OR
           OLD.foto_entrada_url <> NEW.foto_entrada_url OR
           OLD.fecha <> NEW.fecha OR
           OLD.hora_entrada <> NEW.hora_entrada THEN
            RAISE EXCEPTION 'Modificación prohibida: Los parámetros de origen de la asistencia no se pueden alterar.';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_prevent_asistencias_tampering
BEFORE UPDATE OR DELETE ON empleados.empleados_asistencias
FOR EACH ROW EXECUTE FUNCTION empleados.prevent_asistencias_tampering();
