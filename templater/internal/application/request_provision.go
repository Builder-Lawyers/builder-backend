package application

import (
	"builder-templater/internal/dns"
	"builder-templater/internal/presentation/rest"
	"builder-templater/internal/storage"
)

type RequestProvision struct {
	*storage.Storage
	*dns.DNSProvisioner
}

func NewRequestProvision() ProvisionSite {
	return ProvisionSite{
		storage.NewStorage(),
		dns.NewDNSProvisioner(),
	}
}

func (c RequestProvision) Execute(req rest.ProvisionSiteRequest) (uint64, error) {

	return 0, nil
}
