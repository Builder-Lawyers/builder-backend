package commands

import (
	"context"
	"fmt"

	"github.com/Builder-Lawyers/builder-backend/internal/application/dto"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
)

type CreateTemplate struct {
	uowFactory *dbs.UOWFactory
}

func NewCreateTemplate(factory *dbs.UOWFactory) *CreateTemplate {
	return &CreateTemplate{uowFactory: factory}
}

func (c *CreateTemplate) Execute(ctx context.Context, req *dto.CreateTemplateRequest) (uint8, error) {
	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return 0, err
	}

	var templateID uint8
	err = tx.QueryRow(ctx, "INSERT INTO builder.templates(name, fields) VALUES($1, $2) RETURNING id", req.Name, req.Fields).Scan(&templateID)
	if err != nil {
		return 0, fmt.Errorf("err inserting template")
	}

	err = uow.Commit()
	if err != nil {
		return 0, err
	}

	return templateID, nil
}
