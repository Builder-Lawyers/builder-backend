package site

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
	var domainType consts.ProvisionType

	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return 0, err
	}
	defer uow.Finalize(&err)

	err = tx.QueryRow(ctx, "SELECT creator_id, template_id, status, fields from builder.sites WHERE id = $1", siteID).Scan(
		&site.CreatorID,
		&site.TemplateID,
		&site.Status,
		&site.Fields,
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
			if req.DomainType == nil {
				domainType = consts.DefaultDomain
			} else {
				domainType = consts.ProvisionType(*req.DomainType)
			}
			provisionReq := dto.ProvisionSiteRequest{
				SiteID:       siteID,
				DomainType:   domainType,
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
				return 0, err
			}
		}
		return siteID, nil

	}

	_, err = tx.Exec(ctx, "UPDATE builder.sites SET fields = COALESCE($1, fields), file_id = COALESCE($2, file_id), updated_at = $3 WHERE id = $4",
		req.Fields, req.FileID, time.Now(), siteID)
	if err != nil {
		return 0, err
	}

	return siteID, nil
}
