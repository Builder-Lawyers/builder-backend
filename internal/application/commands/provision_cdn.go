package commands

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application/events"
	"github.com/Builder-Lawyers/builder-backend/internal/application/interfaces"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/config"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/dns"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	shared "github.com/Builder-Lawyers/builder-backend/pkg/interfaces"
	"github.com/aws/aws-sdk-go-v2/service/route53domains/types"
)

type ProvisionCDN struct {
	cfg *config.ProvisionConfig
	*dbs.UOWFactory
	interfaces.ProvisionRepo
	*dns.DNSProvisioner
}

func NewProvisionCDN(
	cfg *config.ProvisionConfig, factory *dbs.UOWFactory, provisionRepo interfaces.ProvisionRepo, dns *dns.DNSProvisioner,
) *ProvisionCDN {
	return &ProvisionCDN{
		cfg,
		factory,
		provisionRepo,
		dns,
	}
}

func (c *ProvisionCDN) Handle(event events.ProvisionCDN) (shared.UoW, error) {
	siteID := strconv.FormatUint(event.SiteID, 10)

	status, err := c.DNSProvisioner.GetDomainStatus(event.OperationID)
	switch status {
	case types.OperationStatusSuccessful:
		slog.Info("Requested domain was provisioned for site %v", event.SiteID)
	default:
		slog.Info("Domain is not provisioned yet for site %v", event.SiteID)
		return nil, nil
	}

	timeout := 3 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	// TODO: verify here domain passed, if it is ok
	distributionID, err := c.DNSProvisioner.MapCfDistributionToS3(ctx, "/sites/"+siteID, event.Domain, event.Domain, event.CertificateARN)
	cancel()
	if err != nil {
		return nil, err
	}

	uow := c.UOWFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return nil, err
	}

	provision, err := c.GetProvisionByID(tx, event.SiteID)
	if err != nil {
		return uow, err
	}
	provision.CloudfrontID = distributionID

	_, err = tx.Exec(context.Background(), "UPDATE builder.provisions SET cloudfront_id = $1, updated_at = $2 WHERE site_id = $3",
		provision.CloudfrontID, time.Now(), event.SiteID)
	if err != nil {
		return uow, err
	}

	return uow, nil
}
