package commands

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application/consts"
	"github.com/Builder-Lawyers/builder-backend/internal/application/dto"
	"github.com/Builder-Lawyers/builder-backend/internal/application/interfaces"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/auth"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
)

type UpdateSite struct {
	*dbs.UOWFactory
	interfaces.EventRepo
}

func NewUpdateSite(factory *dbs.UOWFactory, eventRepo interfaces.EventRepo) *UpdateSite {
	return &UpdateSite{UOWFactory: factory, EventRepo: eventRepo}
}

func (c *UpdateSite) Execute(siteID uint64, req *dto.UpdateSiteRequest, identity *auth.Identity) (uint64, error) {
	var site db.Site
	var creator db.User

	uow := c.UOWFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return 0, err
	}
	err = tx.QueryRow(context.Background(), "SELECT creator_id, template_id, status, fields from builder.sites WHERE id = $1", siteID).Scan(
		&site.CreatorID,
		&site.TemplateID,
		&site.Status,
		&site.Fields,
	)
	if err != nil {
		return 0, err
	}
	err = tx.QueryRow(context.Background(), "SELECT id, first_name, second_name, email, created_at from builder.users WHERE id = $1", site.CreatorID).Scan(
		&creator.ID,
		&creator.FirstName,
		&creator.SecondName,
		&creator.Email,
		&creator.CreatedAt,
	)
	if err != nil {
		return 0, err
	}

	if req.NewStatus != nil {
		// Created - request came from templater, site is ready
		// AwaitingProvision - from frontend, all fields are filled in by user
		site.Status = consts.Status(*req.NewStatus)
		_, err = tx.Exec(context.Background(), "UPDATE builder.sites SET status = $1, updated_at = $2 WHERE id = $3", site.Status, time.Now(), siteID)
		if err != nil {
			return 0, err
		}

		if site.Status == consts.AwaitingProvision {
			slog.Info("requesting site provision", "siteID", siteID)
			var templateName string
			err = tx.QueryRow(context.Background(), "SELECT name FROM builder.templates WHERE id = $1", site.TemplateID).Scan(&templateName)
			if err != nil {
				return 0, err
			}
			fields := db.RawMessageToMap(site.Fields)
			if req.Fields != nil {
				fields = *req.Fields
			}
			templaterReq := dto.ProvisionSiteRequest{
				SiteID:       siteID,
				DomainType:   consts.ProvisionType(*req.DomainType),
				TemplateName: templateName,
				Domain:       *req.Domain,
				Fields:       fields,
			}
			payload, err := json.Marshal(templaterReq)
			if err != nil {
				return 0, err
			}
			outbox := db.Outbox{
				Event:     "SiteAwaitingProvision",
				Status:    int(consts.NotProcessed),
				Payload:   json.RawMessage(payload),
				CreatedAt: time.Now(),
			}
			err = tx.QueryRow(context.Background(), "INSERT INTO builder.outbox (event, status, payload, created_at) VALUES ($1,$2,$3,$4) RETURNING id",
				outbox.Event, outbox.Status, outbox.Payload, outbox.CreatedAt).Scan(&outbox.ID)
			if err != nil {
				return 0, err
			}
		}
		if err = uow.Commit(); err != nil {
			return 0, err
		}
		return siteID, nil

	} else if req.Fields != nil {
		// update site's template or/and fields
		_, err = tx.Exec(context.Background(), "UPDATE builder.sites SET fields = $1, updated_at = $2 WHERE id = $3", req.Fields, time.Now(), siteID)
		if err != nil {
			return 0, err
		}
		if err = uow.Commit(); err != nil {
			return 0, err
		}
		// upload new json and rebuild minio prefix
		return siteID, nil
	}

	return siteID, nil
}
