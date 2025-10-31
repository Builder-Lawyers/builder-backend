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
	"github.com/Builder-Lawyers/builder-backend/pkg/db"
	shared "github.com/Builder-Lawyers/builder-backend/pkg/interfaces"
)

type DeactivateSite struct {
	uowFactory      *db.UOWFactory
	dnsProvisioner  *dns.DNSProvisioner
	provisionConfig config.ProvisionConfig
}

func NewDeactivateSite(uowFactory *db.UOWFactory, dnsProvisioner *dns.DNSProvisioner, provisionConfig config.ProvisionConfig,
) *DeactivateSite {
	return &DeactivateSite{uowFactory: uowFactory, dnsProvisioner: dnsProvisioner, provisionConfig: provisionConfig}
}

func (c *DeactivateSite) Handle(ctx context.Context, event events.DeactivateSite) (shared.UoW, error) {
	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return nil, err
	}
	provisionRepo := repo.NewProvisionRepo(tx)
	provision, err := provisionRepo.GetProvisionByID(ctx, event.SiteID)
	if err != nil {
		return uow, fmt.Errorf("error retrieving site's provision, %v", err)
	}

	// TODO: how do i get a baseDomain and a subdomain?
	baseDomain, subdomain, err := getBaseAndSubdomainFromFull(provision.Domain)
	if err != nil {
		return uow, fmt.Errorf("error separating domain, %v", err)
	}

	timeout := 5 * time.Second
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)

	cloudfrontDomain, err := c.dnsProvisioner.WaitAndGetDistribution(timeoutCtx, provision.CloudfrontID)
	cancel()
	if err != nil {
		return uow, err
	}

	err = c.dnsProvisioner.DeleteSubdomain(ctx, baseDomain, subdomain, cloudfrontDomain)
	if err != nil {
		return uow, err
	}

	err = c.dnsProvisioner.DisableDistribution(ctx, provision.CloudfrontID,
		strconv.FormatUint(event.SiteID, 10), c.provisionConfig.Defaults.S3Domain)
	if err != nil {
		return uow, err
	}

	_, err = tx.Exec(ctx, "UPDATE builder.provisions SET status = $1 WHERE site_id = $2", consts.ProvisionStatusDeactivated, event.SiteID)
	if err != nil {
		return nil, fmt.Errorf("err setting provision status to deactivated, %v", err)
	}

	// TODO: based on plan, do different actions. F.e. if plan is with separate domain - deactivate domain
	var userID string
	var firstName sql.NullString
	var secondName sql.NullString
	err = tx.QueryRow(ctx, "SELECT s.creator_id, u.first_name, u.second_name FROM builder.sites s "+
		"LEFT JOIN builder.users u ON s.creator_id = u.id "+
		"WHERE s.id = $1", event.SiteID).Scan(&userID, &firstName, &secondName)
	if err != nil {
		return uow, fmt.Errorf("error getting site creator, %v", err)
	}

	siteDeactivatedData := mail.SiteDeactivatedData{
		CustomerFirstName:  firstName.String,
		CustomerSecondName: secondName.String,
		Year:               strconv.Itoa(time.Now().Year()),
		SiteURL:            provision.Domain,
		Reason:             event.Reason,
	}

	sendMail := events.SendMail{
		UserID:  userID,
		Subject: siteDeactivatedData.GetSubject(),
		Data:    siteDeactivatedData,
	}

	eventRepo := repo.NewEventRepo(tx)
	err = eventRepo.InsertEvent(ctx, sendMail)
	if err != nil {
		return uow, err
	}

	return uow, nil
}

func getBaseAndSubdomainFromFull(domain string) (string, string, error) {
	lastDot := strings.LastIndex(domain, ".")
	if lastDot == -1 {
		return "", "", fmt.Errorf("invalid domain")
	}
	secondLastDot := strings.LastIndex(domain[:lastDot], ".")
	if secondLastDot == -1 {
		return "", "", fmt.Errorf("invalid domain")
	}

	subdomain := domain[:secondLastDot]
	baseDomain := domain[secondLastDot+1:]

	fmt.Println("before:", subdomain)
	fmt.Println("after:", baseDomain)
	return baseDomain, subdomain, nil
}
