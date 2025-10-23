package commands

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application/events"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/config"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db/repo"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/dns"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	shared "github.com/Builder-Lawyers/builder-backend/pkg/interfaces"
	"github.com/aws/aws-sdk-go-v2/service/route53domains/types"
)

type ProvisionCDN struct {
	cfg            *config.ProvisionConfig
	uowFactory     *dbs.UOWFactory
	dnsProvisioner *dns.DNSProvisioner
}

func NewProvisionCDN(
	cfg *config.ProvisionConfig, factory *dbs.UOWFactory, dns *dns.DNSProvisioner,
) *ProvisionCDN {
	return &ProvisionCDN{
		cfg,
		factory,
		dns,
	}
}

func (c *ProvisionCDN) Handle(ctx context.Context, event events.ProvisionCDN) (shared.UoW, error) {
	siteID := strconv.FormatUint(event.SiteID, 10)

	status, err := c.dnsProvisioner.GetDomainStatus(ctx, event.OperationID)
	switch status {
	case types.OperationStatusSuccessful:
		slog.Info("Requested domain was provisioned for site", "siteID", event.SiteID)
	default:
		slog.Info("Domain is not provisioned yet for site", "siteID", event.SiteID)
		return nil, nil
	}

	timeout := 3 * time.Second
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	// TODO: verify here domain passed, if it is ok
	distributionID, err := c.dnsProvisioner.MapCfDistributionToS3(timeoutCtx, "/sites/"+siteID, event.Domain, event.Domain, event.CertificateARN)
	cancel()
	if err != nil {
		return nil, err
	}

	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return nil, err
	}

	provisionRepo := repo.NewProvisionRepo(tx)
	provision, err := provisionRepo.GetProvisionByID(ctx, event.SiteID)
	if err != nil {
		return uow, err
	}
	provision.CloudfrontID = distributionID

	_, err = tx.Exec(ctx, "UPDATE builder.provisions SET cloudfront_id = $1, updated_at = $2 WHERE site_id = $3",
		provision.CloudfrontID, time.Now(), event.SiteID)
	if err != nil {
		return uow, err
	}

	return uow, nil
}
