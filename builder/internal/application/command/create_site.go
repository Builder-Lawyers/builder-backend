package command

import (
	"builder/internal/domain/entity"
	"builder/internal/infra/client/templater"
	"builder/internal/infra/db"
	"builder/internal/presentation/rest"
	"context"
	"fmt"
	"time"
)

type CreateSite struct {
	db.UOWFactory
	templater.TemplaterClient
}

func NewCreateSite(factory db.UOWFactory, client templater.TemplaterClient) CreateSite {
	return CreateSite{UOWFactory: factory, TemplaterClient: client}
}

func (c CreateSite) Execute(req rest.CreateSiteRequest) (uint64, error) {
	var creator entity.User
	uow := c.UOWFactory.GetUoW()

	tx, err := uow.Begin()
	if err != nil {
		return 0, err
	}
	err = tx.QueryRow(context.Background(), "SELECT * FROM users WHERE id=$1", req.UserID).Scan(&creator)
	if err != nil {
		return 0, fmt.Errorf("query failed: %v", err)
	}
	newSite := entity.NewSite(req.TemplateID, creator)

	err = tx.QueryRow(context.Background(),
		"INSERT INTO sites(template_id, creator_id, status, fields, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id",
		newSite.TemplateID, newSite.Creator.ID, newSite.Status, req.Fields, time.Now(), time.Now()).Scan(&newSite.ID)
	if err != nil {
		return 0, fmt.Errorf("insert failed: %v", err)
	}

	if err := uow.Commit(); err != nil {
		return 0, err
	}
	return newSite.ID, nil
}
