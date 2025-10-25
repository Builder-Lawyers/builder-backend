package repo_test

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application/consts"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db/repo"
	"github.com/Builder-Lawyers/builder-backend/internal/testinfra"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/stretchr/testify/require"
)

var uowFactory *dbs.UOWFactory

func TestMain(m *testing.M) {
	ctx := context.Background()

	uowFactory = dbs.NewUoWFactory(testinfra.Pool)
	code := m.Run()

	cleanup(ctx)

	os.Exit(code)
}

func TestInsertProvisionSuccessIfValidFields(t *testing.T) {
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
	provisionRepo := repo.NewProvisionRepo(tx)

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

	provisionRepo := repo.NewProvisionRepo(tx)
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

func cleanup(ctx context.Context) {
	_, err := testinfra.Pool.Exec(ctx, "DELETE FROM builder.provisions")
	if err != nil {
		log.Panicf("err cleaning up repo test %v", err)
	}
}
