package query

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/application/dto"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/application/interfaces"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/config"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/dns"
)

type HealthCheckProvision struct {
	cfg *config.ProvisionConfig
	*dbs.UOWFactory
	interfaces.ProvisionRepo
	*dns.DNSProvisioner
	http.Client
}

func NewHealthCheckProvision(
	cfg *config.ProvisionConfig, factory *dbs.UOWFactory, provisionRepo interfaces.ProvisionRepo, dns *dns.DNSProvisioner,
) *HealthCheckProvision {
	return &HealthCheckProvision{
		cfg,
		factory,
		provisionRepo,
		dns,
		http.Client{Timeout: 4 * time.Second},
	}
}

func (c *HealthCheckProvision) Query(siteID uint64) (dto.HealthcheckProvisionResponseStatus, error) {
	site := strconv.FormatUint(siteID, 10)
	uow := c.UOWFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return "", err
	}
	provision, err := c.ProvisionRepo.GetProvisionByID(tx, site)
	if err != nil {
		slog.Error("site is not provisioned yet, %v", "healthcheck", site)
		return dto.NotProvisioned, nil
	}
	resp, err := c.Client.Get("https://" + provision.Domain)
	if err != nil {
		slog.Error("site is unreachable, %v", "healthcheck", site)
		return dto.Unhealthy, nil
	}
	if resp.StatusCode != 200 {
		slog.Error("error response status from site, %v", "healthcheck", site)
		return dto.Unhealthy, nil
	}

	return dto.Healthy, nil
}
