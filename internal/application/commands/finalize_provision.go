package commands

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application/events"
	"github.com/Builder-Lawyers/builder-backend/internal/domain/consts"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/config"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db/repo"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/dns"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/mail"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/Builder-Lawyers/builder-backend/pkg/interfaces"
)

type FinalizeProvision struct {
	cfg *config.ProvisionConfig
	*dbs.UOWFactory
	*dns.DNSProvisioner
	*repo.EventRepo
}

func NewFinalizeProvision(
	cfg *config.ProvisionConfig, factory *dbs.UOWFactory, dns *dns.DNSProvisioner, eventRepo *repo.EventRepo,
) *FinalizeProvision {
	return &FinalizeProvision{
		cfg,
		factory,
		dns,
		eventRepo,
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

	uow := c.UOWFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return nil, err
	}
	newStatus := consts.Created
	_, err = tx.Exec(context.Background(), "UPDATE builder.sites SET status = $1, updated_at = $2 WHERE id = $3", newStatus, time.Now(), event.SiteID)
	if err != nil {
		return uow, err
	}

	_, err = tx.Exec(context.Background(), "UPDATE builder.provisions SET status = $1, updated_at = $2 WHERE site_id = $3",
		consts.ProvisionStatusProvisioned, time.Now(), event.SiteID)
	if err != nil {
		return uow, err
	}

	var userID string
	mailData := mail.SiteCreatedData{
		SiteURL: event.Domain,
	}
	err = tx.QueryRow(context.Background(), "SELECT s.creator_id, u.first_name, u.second_name "+
		"FROM builder.sites s "+
		"LEFT JOIN builder.users u ON s.creator_id = u.id "+
		"WHERE id = $1", event.SiteID,
	).Scan(&userID, mailData.Name, mailData.Surname)
	if err != nil {
		return uow, err
	}

	sendMailEvent := events.SendMail{
		UserID:  userID,
		Subject: string(mail.SiteCreated),
		Data:    mailData,
	}

	if err = c.EventRepo.InsertEvent(tx, sendMailEvent); err != nil {
		return uow, err
	}

	return uow, nil
}
