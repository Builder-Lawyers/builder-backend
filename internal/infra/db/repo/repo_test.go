package repo

import (
	"context"
	"testing"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/domain/consts"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestInsertProvision(t *testing.T) {
	dbConfig := dbs.NewConfig()
	pool, err := pgxpool.New(context.Background(), dbConfig.GetDSN())
	require.NoError(t, err)
	defer pool.Close()

	tx, err := pool.Begin(context.Background())
	require.NoError(t, err)
	defer tx.Rollback(context.Background())

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

	provisionRepo := NewProvisionRepo()

	err = provisionRepo.InsertProvision(tx, provision)
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
