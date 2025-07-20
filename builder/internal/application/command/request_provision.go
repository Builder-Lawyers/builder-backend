package command

import (
	"github.com/Builder-Lawyers/builder-backend/builder/internal/domain/events"
	"github.com/Builder-Lawyers/builder-backend/builder/internal/infra/client/templater"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
)

type RequestProvision struct {
	dbs.UOWFactory
	templater.TemplaterClient
}

func NewRequestProvision(factory dbs.UOWFactory, client templater.TemplaterClient) UpdateSite {
	return UpdateSite{UOWFactory: factory, TemplaterClient: client}
}

func (h *RequestProvision) Handle(event events.SiteAwaitingProvision) (any, error) {

}
