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
	*dbs.UOWFactory
}

func NewCreateSite(factory *dbs.UOWFactory) *CreateSite {
	return &CreateSite{UOWFactory: factory}
}

func (c *CreateSite) Execute(req *dto.CreateSiteRequest, identity *auth.Identity) (uint64, error) {
	var creator db.User
	uow := c.UOWFactory.GetUoW()

	tx, err := uow.Begin()
	if err != nil {
		return 0, err
	}
	err = tx.QueryRow(context.Background(), "SELECT id, first_name, second_name, email, created_at FROM builder.users WHERE id=$1", req.UserID).Scan(
		&creator.ID,
		&creator.FirstName,
		&creator.SecondName,
		&creator.Email,
		&creator.CreatedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("query failed: %v", err)
	}
	newSite := db.Site{
		TemplateID: req.TemplateID,
		CreatorID:  req.UserID,
		PlanID:     req.PlanID,
		Status:     consts.InCreation,
		Fields:     db.MapToRawMessage(*req.Fields),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	err = tx.QueryRow(context.Background(),
		"INSERT INTO builder.sites(template_id, creator_id, plan_id, status, fields, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id",
		newSite.TemplateID, newSite.CreatorID, newSite.PlanID, newSite.Status, newSite.Fields, newSite.CreatedAt, newSite.UpdatedAt).Scan(&newSite.ID)
	if err != nil {
		return 0, fmt.Errorf("insert failed: %v", err)
	}

	if err = uow.Commit(); err != nil {
		return 0, err
	}
	return newSite.ID, nil
}
