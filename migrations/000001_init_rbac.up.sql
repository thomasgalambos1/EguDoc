CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE roles (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code        VARCHAR(100) UNIQUE NOT NULL,
    label       VARCHAR(200) NOT NULL,
    description TEXT,
    system      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE permissions (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    action      VARCHAR(100) NOT NULL,
    subject     VARCHAR(100) NOT NULL,
    condition   JSONB,
    description TEXT,
    UNIQUE(action, subject, COALESCE(condition::text, ''))
);

CREATE TABLE role_permissions (
    role_id       UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE user_roles (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_subject    VARCHAR(255) NOT NULL,
    role_id         UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    institution_id  UUID,
    compartiment_id UUID,
    granted_by      VARCHAR(255),
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_subject, role_id, COALESCE(institution_id::text,''), COALESCE(compartiment_id::text,''))
);

CREATE INDEX idx_user_roles_subject ON user_roles(user_subject) WHERE active = TRUE;
CREATE INDEX idx_user_roles_institution ON user_roles(institution_id) WHERE active = TRUE;
