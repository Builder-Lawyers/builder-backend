package query

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application/dto"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/auth"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/config"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db/repo"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/dns"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
)

type GetSite struct {
	cfg            *config.ProvisionConfig
	uowFactory     *dbs.UOWFactory
	dnsProvisioner *dns.DNSProvisioner
	client         http.Client
}

func NewGetSite(
	cfg *config.ProvisionConfig, factory *dbs.UOWFactory, dns *dns.DNSProvisioner,
) *GetSite {
	return &GetSite{
		cfg,
		factory,
		dns,
		http.Client{Timeout: 4 * time.Second},
	}
}

func (c *GetSite) Query(ctx context.Context, siteIDParam uint64, identity *auth.Identity) (*dto.GetSiteResponse, error) {
	// TODO: check if user owns this site, etc...
	siteID := strconv.FormatUint(siteIDParam, 10)
	var site db.Site

	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return nil, err
	}
	err = tx.QueryRow(ctx, "SELECT creator_id, template_id, status, fields from builder.sites WHERE id = $1", siteID).Scan(
		&site.CreatorID,
		&site.TemplateID,
		&site.Status,
		&site.Fields,
	)
	if err != nil {
		return nil, err
	}

	response := dto.GetSiteResponse{
		HealthCheckStatus: dto.Healthy,
		CreatedAt:         site.CreatedAt.String(),
	}

	provisionRepo := repo.NewProvisionRepo(tx)
	provision, err := provisionRepo.GetProvisionByID(ctx, siteIDParam)
	if err != nil {
		slog.Error("site is not provisioned yet, %v", "healthcheck", site)
		response.HealthCheckStatus = dto.NotProvisioned
		return &response, err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", "https://"+provision.Domain, http.NoBody)
	if err != nil {
		slog.Error("error creating request to provisioned site", siteID)
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		slog.Error("site is unreachable", "siteID", siteID)
		response.HealthCheckStatus = dto.Unhealthy
		return &response, err
	}
	if resp.StatusCode != 200 {
		slog.Error("error response status from site", "siteID", siteID)
		response.HealthCheckStatus = dto.Unhealthy
		return &response, err
	}

	return &response, nil
}
