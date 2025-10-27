package processors

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application/consts"
	"github.com/Builder-Lawyers/builder-backend/internal/application/events"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/config"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db/repo"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/dns"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/mail"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/Builder-Lawyers/builder-backend/pkg/interfaces"
)

type FinalizeProvision struct {
	cfg         config.ProvisionConfig
	uowFactory  *dbs.UOWFactory
	dnsProvider *dns.DNSProvisioner
}

func NewFinalizeProvision(
	cfg config.ProvisionConfig, factory *dbs.UOWFactory, dns *dns.DNSProvisioner,
) *FinalizeProvision {
	return &FinalizeProvision{
		cfg,
		factory,
		dns,
	}
}

func (c *FinalizeProvision) Handle(ctx context.Context, event events.FinalizeProvision) (interfaces.UoW, error) {
	timeout := 10 * time.Second
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	cfDomain, err := c.dnsProvider.WaitAndGetDistribution(timeoutCtx, event.DistributionID)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("err waiting for deployment of distribution, %v", err)
	}
	var baseDomain string
	firstPart := strings.Index(event.Domain, ".")
	if event.DomainType == consts.DefaultDomain {
		baseDomain = event.Domain[firstPart+1:]
	} else {
		baseDomain = event.Domain
	}

	timeout = 5 * time.Second
	timeoutCtx, cancel = context.WithTimeout(ctx, timeout)
	err = c.dnsProvider.CreateSubdomain(timeoutCtx, baseDomain, event.Domain, cfDomain)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("err creating route53 subdomain, %v", err)
	}

	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return nil, err
	}
	newStatus := consts.SiteStatusCreated
	_, err = tx.Exec(ctx, "UPDATE builder.sites SET status = $1, updated_at = $2 WHERE id = $3", newStatus, time.Now(), event.SiteID)
	if err != nil {
		return uow, fmt.Errorf("error updating site status, %v", err)
	}

	_, err = tx.Exec(ctx, "UPDATE builder.provisions SET status = $1, updated_at = $2 WHERE site_id = $3",
		consts.ProvisionStatusProvisioned, time.Now(), event.SiteID)
	if err != nil {
		return uow, fmt.Errorf("error updating provision's status, %v", err)
	}

	var userID string
	var firstName sql.NullString
	var secondName sql.NullString
	err = tx.QueryRow(ctx, "SELECT s.creator_id, u.first_name, u.second_name "+
		"FROM builder.sites s "+
		"LEFT JOIN builder.users u ON s.creator_id = u.id "+
		"WHERE s.id = $1", event.SiteID,
	).Scan(&userID, &firstName, &secondName)
	if err != nil {
		return uow, fmt.Errorf("error getting mail data, %v", err)
	}

	mailData := mail.SiteCreatedData{
		CustomerFirstName:  firstName.String,
		CustomerSecondName: secondName.String,
		SiteURL:            event.Domain,
		Year:               strconv.Itoa(time.Now().Year()),
	}

	sendMailEvent := events.SendMail{
		UserID:  userID,
		Subject: mailData.GetSubject(),
		Data:    mailData,
	}

	eventRepo := repo.NewEventRepo(tx)
	if err = eventRepo.InsertEvent(ctx, sendMailEvent); err != nil {
		return uow, fmt.Errorf("error inserting mail event, %v", err)
	}

	return uow, nil
}
