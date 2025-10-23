package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application/consts"
	"github.com/Builder-Lawyers/builder-backend/internal/application/dto"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/auth"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
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

	if err = uow.Commit(); err != nil {
		return 0, err
	}
	return newSite.ID, nil
}
