package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/Builder-Lawyers/builder-backend/builder/internal/application/dto"
	"github.com/Builder-Lawyers/builder-backend/builder/internal/domain/entity"
	"github.com/Builder-Lawyers/builder-backend/builder/internal/infra/db"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
)

type CreateSite struct {
	*dbs.UOWFactory
}

func NewCreateSite(factory *dbs.UOWFactory) *CreateSite {
	return &CreateSite{UOWFactory: factory}
}

func (c *CreateSite) Execute(req dto.CreateSiteRequest) (uint64, error) {
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
	newSite := entity.NewSite(req.TemplateID, db.MapUserModelToEntity(creator))

	err = tx.QueryRow(context.Background(),
		"INSERT INTO builder.sites(template_id, creator_id, status, fields, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id",
		newSite.TemplateID, newSite.Creator.ID, newSite.Status, req.Fields, time.Now(), time.Now()).Scan(&newSite.ID)
	if err != nil {
		return 0, fmt.Errorf("insert failed: %v", err)
	}

	if err := uow.Commit(); err != nil {
		return 0, err
	}
	return newSite.ID, nil
}
