package repo

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application/consts"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var pool *pgxpool.Pool
var terminateContainer func()

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:17.2-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		postgres.WithInitScripts("init.sql"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp")),
	)
	if err != nil {
		log.Fatalf("could not start postgres container: %s", err)
	}

	terminateContainer = func() {
		_ = pgContainer.Terminate(ctx)
	}

	// Create pgx pool
	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("could not get connection string: %s", err)
	}

	pool, err = pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("could not create pgx pool: %s", err)
	}

	code := m.Run()

	fmt.Println("Closing pool")
	pool.Close()
	terminateContainer()

	os.Exit(code)
}

func TestInsertProvisionSuccessIfValidFields(t *testing.T) {
	uowFactory := dbs.NewUoWFactory(pool)
	uow := uowFactory.GetUoW()
	tx, err := uow.Begin()
	require.NoError(t, err)
	defer uow.Rollback()

	provision := db.Provision{
		SiteID:         123,
		Type:           consts.DefaultDomain,
		Status:         consts.ProvisionStatusInProcess,
		Domain:         "example.com",
		CertificateARN: "arn:aws:acm:us-east-1:0026",
		CloudfrontID:   "cf123",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	ctx := context.Background()

	provisionRepo := NewProvisionRepo(tx)

	err = provisionRepo.InsertProvision(ctx, provision)
	require.NoError(t, err)

	var count int
	err = tx.QueryRow(
		context.Background(),
		`SELECT COUNT(*) FROM builder.provisions WHERE site_id = $1 AND type = $2 AND domain = $3`,
		provision.SiteID, provision.Type, provision.Domain,
	).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count, "expected one inserted provision")
}

func TestGetProvisionReturnsProvisionIfExists(t *testing.T) {
	uowFactory := dbs.NewUoWFactory(pool)
	uow := uowFactory.GetUoW()
	tx, err := uow.Begin()
	require.NoError(t, err)
	defer uow.Rollback()

	provision := db.Provision{
		SiteID:    1234,
		Type:      consts.DefaultDomain,
		Status:    consts.ProvisionStatusInProcess,
		Domain:    "example.com",
		CreatedAt: time.Now().Truncate(0),
		UpdatedAt: time.Now().Truncate(0),
	}

	provisionRepo := NewProvisionRepo(tx)

	ctx := context.Background()

	err = provisionRepo.InsertProvision(ctx, provision)
	require.NoError(t, err)

	insertedProvision, err := provisionRepo.GetProvisionByID(ctx, 1234)
	require.Nil(t, err)
	require.Equal(t, provision.SiteID, insertedProvision.SiteID)
	require.Equal(t, provision.Type, insertedProvision.Type)
	require.Equal(t, provision.Status, insertedProvision.Status)
	require.Equal(t, provision.Domain, insertedProvision.Domain)

	require.WithinDuration(t, provision.CreatedAt, insertedProvision.CreatedAt, time.Microsecond)
	require.WithinDuration(t, provision.UpdatedAt, insertedProvision.UpdatedAt, time.Microsecond)
	require.NotNil(t, insertedProvision, "expected to be a valid struct")
}
