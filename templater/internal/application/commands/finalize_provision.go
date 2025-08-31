package commands

import (
	"context"
	"log/slog"
	"strings"
	"time"

	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/Builder-Lawyers/builder-backend/pkg/interfaces"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/client/builder"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/config"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/consts"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/dns"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/events"
)

type FinalizeProvision struct {
	cfg *config.ProvisionConfig
	*dbs.UOWFactory
	*dns.DNSProvisioner
	*builder.BuilderClient
}

func NewFinalizeProvision(
	cfg *config.ProvisionConfig, factory *dbs.UOWFactory, dns *dns.DNSProvisioner,
	builderClient *builder.BuilderClient,
) *FinalizeProvision {
	return &FinalizeProvision{
		cfg,
		factory,
		dns,
		builderClient,
	}
}

func (c *FinalizeProvision) Handle(event events.FinalizeProvision) (interfaces.UoW, error) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cfDomain, err := c.DNSProvisioner.WaitAndGetDistribution(ctx, event.DistributionID)
	if err != nil {
		slog.Error("err waiting for deployment of distribution", "cf", err)
		return nil, err
	}
	var baseDomain string
	firstPart := strings.Index(event.Domain, ".")
	if event.DomainType == consts.DefaultDomain {
		baseDomain = event.Domain[firstPart+1:]
	} else {
		baseDomain = event.Domain
	}
	err = c.DNSProvisioner.CreateSubdomain(baseDomain, event.Domain, cfDomain)
	if err != nil {
		slog.Error("err creating route53 subdomain", "r53", err)
		return nil, err
	}

	newStatus := builder.Created
	_, err = c.BuilderClient.UpdateSite(event.SiteID, builder.UpdateSiteRequest{NewStatus: &newStatus})
	if err != nil {
		slog.Error("err updating site's status", "builder", err)
		return nil, err
	}
	uow := c.UOWFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(context.Background(), "UPDATE builder.provisions SET status = $1, updated_at = $2 WHERE site_id = $3",
		consts.ProvisionStatusProvisioned, time.Now(), event.SiteID)
	if err != nil {
		return uow, err
	}

	return uow, nil
}
