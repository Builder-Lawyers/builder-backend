package application

import (
	"github.com/Builder-Lawyers/builder-backend/templater/internal/dns"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/presentation/rest"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/storage"
)

type ProvisionSite struct {
	*storage.Storage
	*dns.DNSProvisioner
}

func NewProvisionSite() ProvisionSite {
	return ProvisionSite{
		storage.NewStorage(),
		dns.NewDNSProvisioner(),
	}
}

func (c ProvisionSite) Execute(req rest.ProvisionSiteRequest) (uint64, error) {

	return 0, nil
}
