package site

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application/consts"
	"github.com/Builder-Lawyers/builder-backend/internal/application/dto"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/auth"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/jackc/pgx/v5"
)

type CreateSite struct {
	uowFactory *dbs.UOWFactory
}

func NewCreateSite(factory *dbs.UOWFactory) *CreateSite {
	return &CreateSite{uowFactory: factory}
}

func (c *CreateSite) Execute(ctx context.Context, req *dto.CreateSiteRequest, identity *auth.Identity) (uint64, error) {
	uow := c.uowFactory.GetUoW()

	tx, err := uow.Begin()
	if err != nil {
		return 0, err
	}
	defer uow.Finalize(&err)

	err = c.checkDuplicateSitesByUser(ctx, tx, identity)
	if err != nil {
		return 0, err
	}

	newSite := db.Site{
		TemplateID: req.TemplateID,
		CreatorID:  identity.UserID,
		PlanID:     req.PlanID,
		Status:     consts.SiteStatusInCreation,
		Fields:     db.MapToRawMessage(*req.Fields),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	insertQuery := "INSERT INTO builder.sites(template_id, creator_id, plan_id, status, fields, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id"
	err = tx.QueryRow(ctx, insertQuery, newSite.TemplateID, newSite.CreatorID, newSite.PlanID, newSite.Status,
		newSite.Fields, newSite.CreatedAt, newSite.UpdatedAt).Scan(&newSite.ID)
	if err != nil {
		return 0, fmt.Errorf("insert failed: %v", err)
	}

	return newSite.ID, nil
}

func (c *CreateSite) checkDuplicateSitesByUser(ctx context.Context, tx pgx.Tx, identity *auth.Identity) error {
	var duplicateID int64
	err := tx.QueryRow(ctx, "SELECT id FROM builder.sites WHERE creator_id = $1 AND created_at > $2",
		identity.UserID, time.Now().Add(-time.Minute*5),
	).Scan(&duplicateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// no duplicates, all good
			return nil
		}
		return fmt.Errorf("can't check for duplicate site, %v", err)
	}

	return fmt.Errorf("site was already created by this user recently")
}
