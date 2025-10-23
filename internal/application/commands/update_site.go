package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application/consts"
	"github.com/Builder-Lawyers/builder-backend/internal/application/dto"
	"github.com/Builder-Lawyers/builder-backend/internal/application/errs"
	"github.com/Builder-Lawyers/builder-backend/internal/application/events"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/auth"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db/repo"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
)

type UpdateSite struct {
	uowFactory *dbs.UOWFactory
}

func NewUpdateSite(factory *dbs.UOWFactory) *UpdateSite {
	return &UpdateSite{uowFactory: factory}
}

func (c *UpdateSite) Execute(ctx context.Context, siteID uint64, req *dto.UpdateSiteRequest, identity *auth.Identity) (uint64, error) {
	var site db.Site

	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return 0, err
	}
	err = tx.QueryRow(ctx, "SELECT creator_id, template_id, status, fields, subscription_id from builder.sites WHERE id = $1", siteID).Scan(
		&site.CreatorID,
		&site.TemplateID,
		&site.Status,
		&site.Fields,
		&site.SubscriptionID,
	)
	if err != nil {
		return 0, err
	}

	if identity.UserID != site.CreatorID {
		return 0, errs.PermissionsError{Err: fmt.Errorf("user requesting action, is not a site's creator")}
	}

	if req.NewStatus != nil {
		// SiteStatusAwaitingProvision - from frontend, all fields are filled in by user
		site.Status = consts.SiteStatus(*req.NewStatus)
		_, err = tx.Exec(ctx, "UPDATE builder.sites SET status = $1, updated_at = $2 WHERE id = $3", site.Status, time.Now(), siteID)
		if err != nil {
			return 0, err
		}

		switch site.Status {
		case consts.SiteStatusAwaitingProvision:
			slog.Info("requesting site provision", "siteID", siteID)
			var templateName string
			err = tx.QueryRow(ctx, "SELECT name FROM builder.templates WHERE id = $1", site.TemplateID).Scan(&templateName)
			if err != nil {
				return 0, err
			}
			fields := db.RawMessageToMap(site.Fields)
			if req.Fields != nil {
				fields = *req.Fields
			}
			provisionReq := dto.ProvisionSiteRequest{
				SiteID:       siteID,
				DomainType:   consts.ProvisionType(*req.DomainType),
				TemplateName: templateName,
				Domain:       *req.Domain,
				Fields:       fields,
			}
			payload, err := json.Marshal(provisionReq)
			if err != nil {
				return 0, err
			}
			outbox := db.Outbox{
				Event:     "SiteAwaitingProvision",
				Status:    int(consts.NotProcessed),
				Payload:   json.RawMessage(payload),
				CreatedAt: time.Now(),
			}
			err = tx.QueryRow(ctx, "INSERT INTO builder.outbox (event, status, payload, created_at) VALUES ($1,$2,$3,$4) RETURNING id",
				outbox.Event, outbox.Status, outbox.Payload, outbox.CreatedAt).Scan(&outbox.ID)
			if err != nil {
				return 0, err
			}
		case consts.SiteStatusAwaitingDeactivation:

			deactivateSiteEvent := events.DeactivateSite{
				SiteID: siteID,
				Reason: "Deactivated due to missing payment",
			}

			eventRepo := repo.NewEventRepo(tx)
			err = eventRepo.InsertEvent(ctx, deactivateSiteEvent)
			if err != nil {
				return 0, fmt.Errorf("error creating deactivate site event, %v", err)
			}
		}

		if err = uow.Commit(); err != nil {
			return 0, err
		}
		return siteID, nil

	} else if req.Fields != nil {
		// update site's template or/and fields
		_, err = tx.Exec(ctx, "UPDATE builder.sites SET fields = $1, updated_at = $2 WHERE id = $3", req.Fields, time.Now(), siteID)
		if err != nil {
			return 0, err
		}
		if err = uow.Commit(); err != nil {
			return 0, err
		}
		// TODO: upload new json and rebuild minio prefix
		return siteID, nil
	}

	return siteID, nil
}
