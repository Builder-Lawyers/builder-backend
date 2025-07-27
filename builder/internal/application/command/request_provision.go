package command

//
//import (
//	"fmt"
//	"github.com/Builder-Lawyers/builder-backend/builder/internal/domain/events"
//	"github.com/Builder-Lawyers/builder-backend/builder/internal/infra/client/templater"
//	"github.com/Builder-Lawyers/builder-backend/builder/internal/infra/db"
//	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
//	"github.com/Builder-Lawyers/builder-backend/pkg/interfaces"
//)
//
//type RequestProvision struct {
//	dbs.UOWFactory
//	templater.TemplaterClient
//}
//
//func NewRequestProvision(factory dbs.UOWFactory, client templater.TemplaterClient) RequestProvision {
//	return RequestProvision{UOWFactory: factory, TemplaterClient: client}
//}
//
//func (h *RequestProvision) Handle(event interfaces.Event) (any, error) {
//	evt, ok := event.(*events.SiteAwaitingProvision)
//	if !ok {
//		return nil, fmt.Errorf("can't cast event to needed type")
//	}
//	req := templater.ProvisionSiteRequest{
//		SiteID:         evt.SiteID,
//		TemplateName:   evt.TemplateName,
//		DomainVariants: evt.DomainVariants,
//		Fields:         db.RawMessageToMap(evt.Fields),
//	}
//	_, err := h.TemplaterClient.ProvisionSite(req)
//	return nil, err
//}
