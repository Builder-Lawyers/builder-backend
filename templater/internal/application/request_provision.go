package application

import (
	"github.com/Builder-Lawyers/builder-backend/templater/internal/dns"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/presentation/rest"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/storage"
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
