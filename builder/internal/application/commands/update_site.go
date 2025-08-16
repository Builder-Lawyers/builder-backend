package commands

import (
	"context"
	"time"

	"github.com/Builder-Lawyers/builder-backend/builder/internal/application/dto"
	"github.com/Builder-Lawyers/builder-backend/builder/internal/domain/consts"
	"github.com/Builder-Lawyers/builder-backend/builder/internal/infra/client/templater"
	"github.com/Builder-Lawyers/builder-backend/builder/internal/infra/db"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
)

type UpdateSite struct {
	*dbs.UOWFactory
	*templater.TemplaterClient
}

func NewUpdateSite(factory *dbs.UOWFactory, client *templater.TemplaterClient) *UpdateSite {
	return &UpdateSite{UOWFactory: factory, TemplaterClient: client}
}

func (c *UpdateSite) Execute(siteID uint64, req dto.UpdateSiteRequest) (uint64, error) {
	var siteModel db.Site
	var creator db.User

	uow := c.UOWFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return 0, err
	}
	err = tx.QueryRow(context.Background(), "SELECT creator_id, template_id, status, fields from builder.sites WHERE id = $1", siteID).Scan(
		&siteModel.CreatorID,
		&siteModel.TemplateID,
		&siteModel.Status,
		&siteModel.Fields,
	)
	if err != nil {
		return 0, err
	}
	err = tx.QueryRow(context.Background(), "SELECT id, first_name, second_name, email, created_at from builder.users WHERE id = $1", siteModel.CreatorID).Scan(
		&creator.ID,
		&creator.FirstName,
		&creator.SecondName,
		&creator.Email,
		&creator.CreatedAt,
	)
	if err != nil {
		return 0, err
	}
	site := db.MapSiteModelToEntity(siteModel, creator)

	if req.NewStatus != nil {
		// Created - request came from templater, site is ready
		// AwaitingProvision - from frontend, all fields are filled in by user
		site.UpdateState(consts.Status(*req.NewStatus))
		_, err = tx.Exec(context.Background(), "UPDATE builder.sites SET status = $1, updated_at = $2 WHERE id = $3", site.Status, time.Now(), siteID)
		if err != nil {
			return 0, err
		}
		//// TODO: Move this logic to domain
		//if oldSite.Status == consts.AwaitingProvision {
		//	siteProvision := db.Outbox{
		//		Event:     "SiteAwaitingProvision",
		//		Status:    int(consts.NotProcessed),
		//		CreatedAt: time.Now(),
		//	}
		//	_, err = tx.Exec(context.Background(), "INSERT INTO builder.outbox(event, status, created_at) VALUES ($1, $2, $3)",
		//		siteProvision.Event, siteProvision.Status, siteProvision.CreatedAt)
		//	if err != nil {
		//		return 0, err
		//	}
		//}
		if site.Status == consts.AwaitingProvision {
			var templateName string
			err = tx.QueryRow(context.Background(), "SELECT name FROM builder.templates WHERE id = $1", site.TemplateID).Scan(&templateName)
			if err != nil {
				return 0, err
			}
			fields := db.RawMessageToMap(siteModel.Fields)
			if req.Fields != nil {
				fields = *req.Fields
			}
			templaterReq := templater.ProvisionSiteRequest{
				SiteID:        site.ID,
				ProvisionType: templater.ProvisionSiteRequestProvisionType(*req.DomainType),
				TemplateName:  templateName,
				Domain:        *req.Domain,
				Fields:        fields,
			}
			_, err = c.TemplaterClient.ProvisionSite(templaterReq)
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
