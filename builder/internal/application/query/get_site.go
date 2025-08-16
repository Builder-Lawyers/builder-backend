package query

import (
	"github.com/Builder-Lawyers/builder-backend/builder/internal/infra/client/templater"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
)

type GetSite struct {
	*dbs.UOWFactory
	*templater.TemplaterClient
}

func NewGetSite(factory *dbs.UOWFactory, templaterClient *templater.TemplaterClient) *GetSite {
	return &GetSite{UOWFactory: factory, TemplaterClient: templaterClient}
}

func (c *GetSite) Execute(siteID uint64) (templater.HealthcheckProvisionResponseStatus, error) {
	// check if user owns this site, etc...
	status, err := c.TemplaterClient.HealthCheckSite(siteID)
	if err != nil {
		return "", err
	}

	return status, nil
}
