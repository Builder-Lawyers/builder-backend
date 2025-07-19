package command

import (
	"context"
	"github.com/Builder-Lawyers/builder-backend/builder/internal/application/dto"
	"github.com/Builder-Lawyers/builder-backend/builder/internal/domain/consts"
	"github.com/Builder-Lawyers/builder-backend/builder/internal/infra/client/templater"
	"github.com/Builder-Lawyers/builder-backend/builder/internal/infra/db"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	"time"
)

type UpdateSite struct {
	dbs.UOWFactory
	templater.TemplaterClient
}

func NewUpdateSite(factory dbs.UOWFactory, client templater.TemplaterClient) UpdateSite {
	return UpdateSite{UOWFactory: factory, TemplaterClient: client}
}

func (c UpdateSite) Execute(siteID uint64, req dto.UpdateSiteRequest) (uint64, error) {
	var oldSiteModel db.Site
	var creator db.User

	uow := c.UOWFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return 0, err
	}
	err = tx.QueryRow(context.Background(), "SELECT creator_id, template_id, status from builder.sites WHERE id = $1", siteID).Scan(&oldSiteModel)
	if err != nil {
		return 0, err
	}
	err = tx.QueryRow(context.Background(), "SELECT id, first_name, second_name, email, created_at from builder.users WHERE id = $1", oldSiteModel.CreatorID).Scan(&creator)
	if err != nil {
		return 0, err
	}
	oldSite := db.MapSiteModelToEntity(oldSiteModel, creator)

	if req.NewStatus != nil {
		// Created - request came from templater, site is ready
		// AwaitingProvision - from frontend, all fields are filled in by user
		oldSite.UpdateState(consts.Status(*req.NewStatus))
		_, err = tx.Exec(context.Background(), "UPDATE builder.sites SET status = $1, updated_at = $2 WHERE id = $3", oldSite.Status, time.Now(), siteID)
		if err != nil {
			return 0, err
		}
		// TODO: Move this logic to domain
		if oldSite.Status == consts.AwaitingProvision {
			siteProvision := db.Outbox{
				Event:     "SiteAwaitingProvision",
				Status:    int(consts.NotProcessed),
				CreatedAt: time.Now(),
			}
			_, err = tx.Exec(context.Background(), "INSERT INTO builder.outbox(event, status, created_at) VALUES ($1, $2, $3)",
				siteProvision.Event, siteProvision.Status, siteProvision.CreatedAt)
			if err != nil {
				return 0, err
			}
		}
		if err := uow.Commit(); err != nil {
			return 0, err
		}
		return siteID, nil

	} else if req.Fields != nil {
		// update site's template or/and fields
		_, err = tx.Exec(context.Background(), "UPDATE builder.sites SET fields = $1, updated_at = $2 WHERE id = $3", req.Fields, time.Now(), siteID)
		if err != nil {
			return 0, err
		}
		if err := uow.Commit(); err != nil {
			return 0, err
		}
		return siteID, nil
	}

	return siteID, nil
}
