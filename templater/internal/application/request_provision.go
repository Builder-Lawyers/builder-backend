package application

import (
	"context"
	"encoding/json"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/application/dto"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/consts"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/db"
	"log"
	"time"
)

type RequestProvision struct {
	*dbs.UOWFactory
}

func NewRequestProvision(factory *dbs.UOWFactory) *RequestProvision {
	return &RequestProvision{
		factory,
	}
}

func (c *RequestProvision) Execute(req dto.ProvisionSiteRequest) (uint64, error) {
	//event := events.SiteAwaitingProvision{
	//	SiteID:         req.SiteID,
	//	TemplateName:   req.TemplateName,
	//	DomainVariants: req.DomainVariants,
	//	Fields:         db.MapToRawMessage(req.Fields),
	//	CreatedAt:      time.Now(),
	//}
	payload, err := json.Marshal(req)
	if err != nil {
		log.Println(err)
		return 0, err
	}
	outbox := db.Outbox{
		Event:     "SiteAwaitingProvision",
		Status:    consts.NotProcessed,
		Payload:   json.RawMessage(payload),
		CreatedAt: time.Now(),
	}
	uow := c.UOWFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return 0, err
	}
	err = tx.QueryRow(context.Background(), "INSERT INTO builder.outbox (event, status, payload, created_at) VALUES ($1,$2,$3,$4) RETURNING id",
		outbox.Event, outbox.Status, outbox.Payload, outbox.CreatedAt).Scan(&outbox.ID)
	if err != nil {
		return 0, err
	}
	if err = tx.Commit(context.Background()); err != nil {
		return 0, err
	}

	return outbox.ID, nil
}
