-- Migration 001: Create catalogos schema with all master-data tables
-- All FKs use ON DELETE RESTRICT (never CASCADE)
-- All tables include soft-delete via deleted_at

CREATE SCHEMA IF NOT EXISTS catalogos;

CREATE OR REPLACE FUNCTION catalogos.trigger_set_timestamp()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = CURRENT_TIMESTAMP;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ============================================================
-- GEOGRAPHY
-- ============================================================

CREATE TABLE IF NOT EXISTS catalogos.catalog_countries (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code       VARCHAR(2)   UNIQUE NOT NULL,
    name       VARCHAR(100) UNIQUE NOT NULL,
    phone_code VARCHAR(10)  NOT NULL,
    deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS catalogos.catalog_states (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    country_id BIGINT       NOT NULL,
    code       VARCHAR(10)  NOT NULL,
    name       VARCHAR(100) NOT NULL,
    deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT fk_states_country   FOREIGN KEY (country_id) REFERENCES catalogos.catalog_countries(id) ON DELETE RESTRICT,
    CONSTRAINT uq_state_per_country UNIQUE (country_id, name)
);

CREATE TABLE IF NOT EXISTS catalogos.catalog_cities (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    state_id   BIGINT       NOT NULL,
    name       VARCHAR(100) NOT NULL,
    deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT fk_cities_state  FOREIGN KEY (state_id) REFERENCES catalogos.catalog_states(id) ON DELETE RESTRICT,
    CONSTRAINT uq_city_per_state UNIQUE (state_id, name)
);

CREATE TABLE IF NOT EXISTS catalogos.catalog_locality_types (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code       VARCHAR(20)  UNIQUE NOT NULL,
    name       VARCHAR(100) NOT NULL,
    deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- ============================================================
-- FINANCIAL / FISCAL
-- ============================================================

CREATE TABLE IF NOT EXISTS catalogos.catalog_banks (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code       VARCHAR(10)  UNIQUE NOT NULL,
    name       VARCHAR(150) UNIQUE NOT NULL,
    deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS catalogos.catalog_tax_regimes (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code            VARCHAR(10)  UNIQUE NOT NULL,
    name            VARCHAR(200) NOT NULL,
    persona_fisica  BOOLEAN DEFAULT TRUE NOT NULL,
    persona_moral   BOOLEAN DEFAULT TRUE NOT NULL,
    deleted_at      TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS catalogos.catalog_payment_forms (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code       VARCHAR(10)  UNIQUE NOT NULL,
    name       VARCHAR(100) NOT NULL,
    deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS catalogos.catalog_payment_conditions (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code       VARCHAR(50)  UNIQUE NOT NULL,
    name       VARCHAR(100) NOT NULL,
    days       INT DEFAULT 0 NOT NULL CHECK (days >= 0),
    deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- ============================================================
-- OPERATIONAL / SYSTEM
-- ============================================================

CREATE TABLE IF NOT EXISTS catalogos.catalog_workflow_statuses (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code       VARCHAR(50)  UNIQUE NOT NULL,
    name       VARCHAR(100) NOT NULL,
    role_id    SMALLINT     NOT NULL,
    deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS catalogos.catalog_complaint_types (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code        VARCHAR(50)  UNIQUE NOT NULL,
    name        VARCHAR(100) NOT NULL,
    description TEXT,
    deleted_at  TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS catalogos.catalog_services (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code        VARCHAR(50)   UNIQUE NOT NULL,
    name        VARCHAR(150)  NOT NULL,
    description TEXT,
    price       NUMERIC(12,2) DEFAULT 0.00 NOT NULL CHECK (price >= 0.00),
    is_active   BOOLEAN DEFAULT TRUE NOT NULL,
    deleted_at  TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS catalogos.catalog_subscription_plans (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code       VARCHAR(50)   UNIQUE NOT NULL,
    name       VARCHAR(100)  NOT NULL,
    amount     NUMERIC(12,2) NOT NULL CHECK (amount >= 0.00),
    deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS catalogos.catalog_date_periodicities (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code       VARCHAR(50)  UNIQUE NOT NULL,
    name       VARCHAR(100) NOT NULL,
    deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- ============================================================
-- HR / JOBS
-- ============================================================

CREATE TABLE IF NOT EXISTS catalogos.catalog_hr_absence_types (
    id                    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code                  VARCHAR(50)  UNIQUE NOT NULL,
    name                  VARCHAR(100) NOT NULL,
    requires_justification BOOLEAN DEFAULT TRUE NOT NULL,
    deleted_at            TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    created_at            TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at            TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS catalogos.catalog_job_categories (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code       VARCHAR(50)  UNIQUE NOT NULL,
    name       VARCHAR(100) UNIQUE NOT NULL,
    deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT chk_job_categories_lowercase_name CHECK (name = LOWER(name))
);

CREATE TABLE IF NOT EXISTS catalogos.catalog_job_types (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code       VARCHAR(50)  UNIQUE NOT NULL,
    name       VARCHAR(100) UNIQUE NOT NULL,
    deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT chk_job_types_lowercase_name CHECK (name = LOWER(name))
);

-- ============================================================
-- INDEXES
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_states_country_id       ON catalogos.catalog_states(country_id);
CREATE INDEX IF NOT EXISTS idx_cities_state_id         ON catalogos.catalog_cities(state_id);
CREATE INDEX IF NOT EXISTS idx_workflow_statuses_role  ON catalogos.catalog_workflow_statuses(role_id);

-- Soft-delete query support
CREATE INDEX IF NOT EXISTS idx_countries_deleted_at         ON catalogos.catalog_countries(deleted_at)          WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_states_deleted_at            ON catalogos.catalog_states(deleted_at)             WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_cities_deleted_at            ON catalogos.catalog_cities(deleted_at)             WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_locality_types_deleted_at    ON catalogos.catalog_locality_types(deleted_at)     WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_banks_deleted_at             ON catalogos.catalog_banks(deleted_at)              WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_tax_regimes_deleted_at       ON catalogos.catalog_tax_regimes(deleted_at)        WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_payment_forms_deleted_at     ON catalogos.catalog_payment_forms(deleted_at)      WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_payment_conditions_deleted_at ON catalogos.catalog_payment_conditions(deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_workflow_statuses_deleted_at ON catalogos.catalog_workflow_statuses(deleted_at)  WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_complaint_types_deleted_at   ON catalogos.catalog_complaint_types(deleted_at)    WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_services_deleted_at          ON catalogos.catalog_services(deleted_at)           WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_subscription_plans_deleted_at ON catalogos.catalog_subscription_plans(deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_date_periodicities_deleted_at ON catalogos.catalog_date_periodicities(deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_hr_absence_types_deleted_at  ON catalogos.catalog_hr_absence_types(deleted_at)   WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_job_categories_deleted_at    ON catalogos.catalog_job_categories(deleted_at)     WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_job_types_deleted_at         ON catalogos.catalog_job_types(deleted_at)          WHERE deleted_at IS NULL;

-- ============================================================
-- AUTO-UPDATE TRIGGERS (all 15 tables)
-- ============================================================

CREATE TRIGGER set_timestamp_countries
    BEFORE UPDATE ON catalogos.catalog_countries
    FOR EACH ROW EXECUTE FUNCTION catalogos.trigger_set_timestamp();

CREATE TRIGGER set_timestamp_states
    BEFORE UPDATE ON catalogos.catalog_states
    FOR EACH ROW EXECUTE FUNCTION catalogos.trigger_set_timestamp();

CREATE TRIGGER set_timestamp_cities
    BEFORE UPDATE ON catalogos.catalog_cities
    FOR EACH ROW EXECUTE FUNCTION catalogos.trigger_set_timestamp();

CREATE TRIGGER set_timestamp_locality_types
    BEFORE UPDATE ON catalogos.catalog_locality_types
    FOR EACH ROW EXECUTE FUNCTION catalogos.trigger_set_timestamp();

CREATE TRIGGER set_timestamp_banks
    BEFORE UPDATE ON catalogos.catalog_banks
    FOR EACH ROW EXECUTE FUNCTION catalogos.trigger_set_timestamp();

CREATE TRIGGER set_timestamp_tax_regimes
    BEFORE UPDATE ON catalogos.catalog_tax_regimes
    FOR EACH ROW EXECUTE FUNCTION catalogos.trigger_set_timestamp();

CREATE TRIGGER set_timestamp_payment_forms
    BEFORE UPDATE ON catalogos.catalog_payment_forms
    FOR EACH ROW EXECUTE FUNCTION catalogos.trigger_set_timestamp();

CREATE TRIGGER set_timestamp_payment_conditions
    BEFORE UPDATE ON catalogos.catalog_payment_conditions
    FOR EACH ROW EXECUTE FUNCTION catalogos.trigger_set_timestamp();

CREATE TRIGGER set_timestamp_workflow_statuses
    BEFORE UPDATE ON catalogos.catalog_workflow_statuses
    FOR EACH ROW EXECUTE FUNCTION catalogos.trigger_set_timestamp();

CREATE TRIGGER set_timestamp_complaint_types
    BEFORE UPDATE ON catalogos.catalog_complaint_types
    FOR EACH ROW EXECUTE FUNCTION catalogos.trigger_set_timestamp();

CREATE TRIGGER set_timestamp_services
    BEFORE UPDATE ON catalogos.catalog_services
    FOR EACH ROW EXECUTE FUNCTION catalogos.trigger_set_timestamp();

CREATE TRIGGER set_timestamp_subscription_plans
    BEFORE UPDATE ON catalogos.catalog_subscription_plans
    FOR EACH ROW EXECUTE FUNCTION catalogos.trigger_set_timestamp();

CREATE TRIGGER set_timestamp_date_periodicities
    BEFORE UPDATE ON catalogos.catalog_date_periodicities
    FOR EACH ROW EXECUTE FUNCTION catalogos.trigger_set_timestamp();

CREATE TRIGGER set_timestamp_hr_absence_types
    BEFORE UPDATE ON catalogos.catalog_hr_absence_types
    FOR EACH ROW EXECUTE FUNCTION catalogos.trigger_set_timestamp();

CREATE TRIGGER set_timestamp_job_categories
    BEFORE UPDATE ON catalogos.catalog_job_categories
    FOR EACH ROW EXECUTE FUNCTION catalogos.trigger_set_timestamp();

CREATE TRIGGER set_timestamp_job_types
    BEFORE UPDATE ON catalogos.catalog_job_types
    FOR EACH ROW EXECUTE FUNCTION catalogos.trigger_set_timestamp();
