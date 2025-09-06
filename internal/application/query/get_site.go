package query

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application/dto"
	"github.com/Builder-Lawyers/builder-backend/internal/application/interfaces"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/auth"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/config"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/dns"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
)

type GetSite struct {
	cfg *config.ProvisionConfig
	*dbs.UOWFactory
	interfaces.ProvisionRepo
	*dns.DNSProvisioner
	http.Client
}

func NewGetSite(
	cfg *config.ProvisionConfig, factory *dbs.UOWFactory, provisionRepo interfaces.ProvisionRepo, dns *dns.DNSProvisioner,
) *GetSite {
	return &GetSite{
		cfg,
		factory,
		provisionRepo,
		dns,
		http.Client{Timeout: 4 * time.Second},
	}
}

func (c *GetSite) Query(siteIDParam uint64, identity *auth.Identity) (dto.GetSiteResponse, error) {
	// TODO: check if user owns this site, etc...
	siteID := strconv.FormatUint(siteIDParam, 10)
	var site db.Site

	uow := c.UOWFactory.GetUoW()
	tx, err := uow.Begin()
	err = tx.QueryRow(context.Background(), "SELECT creator_id, template_id, status, fields from builder.sites WHERE id = $1", siteID).Scan(
		&site.CreatorID,
		&site.TemplateID,
		&site.Status,
		&site.Fields,
	)
	if err != nil {
		return dto.GetSiteResponse{}, err
	}

	var healthCheckStatus dto.GetSiteResponseHealthCheckStatus
	provision, err := c.ProvisionRepo.GetProvisionByID(tx, siteID)
	if err != nil {
		slog.Error("site is not provisioned yet, %v", "healthcheck", site)
		healthCheckStatus = dto.NotProvisioned
	}
	resp, err := c.Client.Get("https://" + provision.Domain)
	if err != nil {
		slog.Error("site is unreachable, %v", "healthcheck", site)
		healthCheckStatus = dto.Unhealthy
	}
	if resp.StatusCode != 200 {
		slog.Error("error response status from site, %v", "healthcheck", site)
		healthCheckStatus = dto.Unhealthy
	}

	return dto.GetSiteResponse{
		HealthCheckStatus: healthCheckStatus,
		CreatedAt:         site.CreatedAt.String(),
	}, nil
}
