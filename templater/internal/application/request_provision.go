package application

import (
	"github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/presentation/rest"
)

type RequestProvision struct {
	db.UOWFactory
}

func NewRequestProvision(factory db.UOWFactory) RequestProvision {
	return RequestProvision{
		factory,
	}
}

func (c RequestProvision) Execute(req rest.ProvisionSiteRequest) (uint64, error) {

	return 0, nil
}
