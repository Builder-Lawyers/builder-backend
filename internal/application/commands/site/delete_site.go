package site

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application/errs"
	"github.com/Builder-Lawyers/builder-backend/internal/application/events"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/auth"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db/repo"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
)

type DeleteSite struct {
	uowFactory *dbs.UOWFactory
}

func NewDeleteSite(UOWFactory *dbs.UOWFactory) *DeleteSite {
	return &DeleteSite{uowFactory: UOWFactory}
}

func (c *DeleteSite) Execute(ctx context.Context, siteID uint64, identity *auth.Identity) error {

	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return err
	}
	defer uow.Finalize(&err)

	var createdAt time.Time
	err = tx.QueryRow(ctx, "SELECT created_at FROM builder.sites WHERE id = $1 AND creator_id = $2",
		siteID, identity.UserID).Scan(&createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errs.PermissionsError{Err: err}
		}
		return fmt.Errorf("err checking if user owns site, %v", err)
	}

	deactivateSiteEvent := events.DeactivateSite{
		SiteID: siteID,
		Reason: "Site deactivation was requested by it's owner",
	}

	eventRepo := repo.NewEventRepo(tx)
	err = eventRepo.InsertEvent(ctx, deactivateSiteEvent)
	if err != nil {
		return err
	}

	return nil
}
