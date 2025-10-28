package testinfra

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var Pool *pgxpool.Pool
var AwsCfg aws.Config

func init() {
	Pool = SetupDB()
	AwsCfg = SetupAWS()
}

func SetupDB() *pgxpool.Pool {

	ctx := context.Background()

	pgReq := testcontainers.ContainerRequest{
		Image:        "postgres:17.2-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_PASSWORD": "password",
			"POSTGRES_USER":     "postgres",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections"),
	}
	pgC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: pgReq,
		Started:          true,
	})
	if err != nil {
		log.Panicf("start postgres: %v", err)
	}

	pgHostPort, err := pgC.Endpoint(ctx, "")
	if err != nil {
		log.Panicf("postgres endpoint: %v", err)
	}
	pgDSN := fmt.Sprintf("postgres://postgres:password@%s/testdb?sslmode=disable", pgHostPort)

	pool, err := pgxpool.New(ctx, pgDSN)
	if err != nil {
		log.Panicf("pgxpool connect: %v", err)
	}

	ok := false
	for i := 0; i < 20; i++ {
		slog.Info("ping db", "try", i)
		ctxPing, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		err = pool.Ping(ctxPing)
		cancel()
		if err == nil {
			ok = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ok {
		log.Panic("db did not respond after 20 attempts")
	}

	_, err = pool.Exec(ctx, `
		CREATE SCHEMA IF NOT EXISTS builder;
		CREATE TABLE IF NOT EXISTS builder.users (
		  id UUID PRIMARY KEY,
		  email TEXT UNIQUE NOT NULL
		);
		CREATE TABLE IF NOT EXISTS builder.sessions (
		  id UUID PRIMARY KEY,
		  user_id UUID NOT NULL REFERENCES builder.users(id),
		  refresh_token TEXT,
		  issued_at TIMESTAMP WITH TIME ZONE
		);
		CREATE TABLE IF NOT EXISTS builder.files (
			id UUID PRIMARY KEY
		);
		CREATE TABLE IF NOT EXISTS builder.provisions (
			site_id BIGINT PRIMARY KEY,
			"type" VARCHAR(40) NOT NULL,
			status VARCHAR(40) NOT NULL,
			domain VARCHAR(80),
			cert_arn VARCHAR(120),
			cloudfront_id VARCHAR(60),
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ
		);
	`)
	if err != nil {
		log.Panicf("create tables: %v", err)
	}

	return pool
}

func SetupAWS() aws.Config {
	slog.Info("SETUP AWS CONFIG")
	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Panic("can't load aws config", err)
	}

	return awsCfg
}
