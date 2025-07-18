package application

import (
	"builder-templater/internal/dns"
	"builder-templater/internal/presentation/rest"
	"builder-templater/internal/storage"
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
