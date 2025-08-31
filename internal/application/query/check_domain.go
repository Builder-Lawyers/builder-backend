package query

import (
	"github.com/Builder-Lawyers/builder-backend/templater/internal/dns"
)

type CheckDomain struct {
	*dns.DNSProvisioner
}

func NewCheckDomain(dns *dns.DNSProvisioner) *CheckDomain {
	return &CheckDomain{
		dns,
	}
}

func (c *CheckDomain) Query(domain string) (bool, error) {
	return c.CheckAvailability(domain)
}
