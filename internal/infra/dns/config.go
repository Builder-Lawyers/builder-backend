package dns

import (
	"os"
)

type DomainContact struct {
	FirstName    string
	LastName     string
	Email        string
	PhoneNumber  string
	AddressLine1 string
	City         string
	State        string
	CountryCode  string
	ZipCode      string
}

func NewDomainContact() *DomainContact {
	return &DomainContact{
		FirstName:    os.Getenv("DOMAIN_FN"),
		LastName:     os.Getenv("DOMAIN_LN"),
		Email:        os.Getenv("DOMAIN_E"),
		PhoneNumber:  os.Getenv("DOMAIN_PN"),
		AddressLine1: os.Getenv("DOMAIN_AL"),
		City:         os.Getenv("DOMAIN_C"),
		State:        os.Getenv("DOMAIN_S"),
		CountryCode:  os.Getenv("DOMAIN_CC"),
		ZipCode:      os.Getenv("DOMAIN_ZC"),
	}
}
