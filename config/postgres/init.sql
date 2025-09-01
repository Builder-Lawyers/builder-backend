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
    status SMALLINT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS builder.provisions (
    site_id BIGINT PRIMARY KEY,
    type VARCHAR(40) NOT NULL,
    status VARCHAR(40) NOT NULL,
    domain VARCHAR(80),
    cert_arn VARCHAR(120),
    cloudfront_id VARCHAR(60),
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS builder.mails (
    id BIGINT GENERATED ALWAYS AS IDENTITY,
    type VARCHAR(60) NOT NULL,
    recipients VARCHAR(255) NOT NULL,
    subject VARCHAR(100),
    content TEXT,
    sent_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS builder.mail_templates (
    id SMALLINT GENERATED ALWAYS AS IDENTITY,
    type VARCHAR(60) NOT NULL,
    content TEXT
);

insert into builder.users (id, first_name, second_name, email, created_at) values ('5a9bf3fa-d99a-4ccc-b64f-b2ddf20ee5e5', 'John', 'Doe', 'example@gmail.com', CURRENT_TIMESTAMP);
insert into builder.templates(id, name) VALUES (1, 'template-v1');
insert into builder.templates(id, name) VALUES (2, 'template-v2');