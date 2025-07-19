CREATE SCHEMA IF NOT EXISTS builder;
CREATE SCHEMA IF NOT EXISTS keycloak;

CREATE TABLE IF NOT EXISTS builder.sites (
    id BIGINT GENERATED ALWAYS AS IDENTITY,
    template_id SMALLINT NOT NULL,
    creator_id UUID NOT NULL,
    status VARCHAR(30) NOT NULL,
    fields JSONB,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS builder.users (
    id UUID PRIMARY KEY,
    first_name varchar(100) NOT NULL,
    second_name varchar(100) NOT NULL,
    email varchar(100) NOT NULL,
    created_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS builder.templates (
    id SMALLINT PRIMARY KEY,
    name VARCHAR(100) NOT NULL
);

CREATE TABLE IF NOT EXISTS builder.outbox (
    id BIGINT GENERATED ALWAYS AS IDENTITY,
    event VARCHAR(200) NOT NULL,
    status VARCHAR(60) NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);