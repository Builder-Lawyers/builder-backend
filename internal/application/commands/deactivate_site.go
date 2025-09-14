package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application/events"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db/repo"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/dns"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/mail"
	"github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/Builder-Lawyers/builder-backend/pkg/interfaces"
)

type DeactivateSite struct {
	uowFactory     *db.UOWFactory
	dnsProvisioner *dns.DNSProvisioner
	provisionRepo  *repo.ProvisionRepo
	eventRepo      *repo.EventRepo
}

func NewDeactivateSite(uowFactory *db.UOWFactory, dnsProvisioner *dns.DNSProvisioner,
	provisionRepo *repo.ProvisionRepo, eventRepo *repo.EventRepo,
) *DeactivateSite {
	return &DeactivateSite{uowFactory: uowFactory, dnsProvisioner: dnsProvisioner,
		provisionRepo: provisionRepo, eventRepo: eventRepo,
	}
}

func (c *DeactivateSite) Handle(event events.DeactivateSite) (interfaces.UoW, error) {
	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return nil, err
	}
	provision, err := c.provisionRepo.GetProvisionByID(tx, event.SiteID)
	if err != nil {
		return uow, fmt.Errorf("error retrieving site's provision, %v", err)
	}

	// TODO: how do i get a baseDomain and a subdomain?
	baseDomain, subdomain, err := getBaseAndSubdomainFromFull(provision.Domain)
	if err != nil {
		return uow, fmt.Errorf("error separating domain, %v", err)
	}

	timeout := 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	cloudfrontDomain, err := c.dnsProvisioner.WaitAndGetDistribution(ctx, provision.CloudfrontID)
	cancel()
	if err != nil {
		return uow, err
	}

	err = c.dnsProvisioner.DeleteSubdomain(ctx, baseDomain, subdomain, cloudfrontDomain)
	if err != nil {
		return uow, err
	}

	err = c.dnsProvisioner.DisableDistribution(ctx, provision.CloudfrontID)
	if err != nil {
		return uow, err
	}

	siteDeactivatedData := mail.SiteDeactivatedData{
		Year:    strconv.Itoa(time.Now().Year()),
		SiteURL: provision.Domain,
		Reason:  event.Reason,
	}

	var userID string
	err = tx.QueryRow(ctx, "SELECT s.creator_id, u.first_name, u.second_name FROM builder.sites s "+
		"LEFT JOIN builder.users u ON s.creator_id = u.id "+
		"WHERE id = $1", event.SiteID).Scan(&userID, &siteDeactivatedData.CustomerFirstName, &siteDeactivatedData.CustomerSecondName)
	if err != nil {
		return uow, fmt.Errorf("error getting site creator, %v", err)
	}

	sendMail := events.SendMail{
		UserID:  userID,
		Subject: siteDeactivatedData.GetSubject(),
		Data:    siteDeactivatedData,
	}

	err = c.eventRepo.InsertEvent(tx, sendMail)
	if err != nil {
		return uow, fmt.Errorf("error creating event, %v", err)
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
