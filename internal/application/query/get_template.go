package query

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/Builder-Lawyers/builder-backend/internal/application/dto"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/config"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/storage"
	"github.com/Builder-Lawyers/builder-backend/pkg/db"
)

type GetTemplate struct {
	uowFactory *db.UOWFactory
	storage    *storage.Storage
	cfg        *config.ProvisionConfig
}

func NewGetTemplate(uowFactory *db.UOWFactory, storage *storage.Storage, provisionConfig *config.ProvisionConfig) *GetTemplate {
	return &GetTemplate{uowFactory: uowFactory, storage: storage, cfg: provisionConfig}
}

func (c *GetTemplate) Query(ctx context.Context, templateID uint8) (*dto.TemplateInfo, error) {

	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return nil, err
	}

	var templateName string
	err = tx.QueryRow(ctx, "SELECT name FROM builder.templates WHERE id = $1", templateID).Scan(&templateName)
	if err != nil {
		return nil, fmt.Errorf("err getting template name")
	}

	fieldsFile, err := c.storage.GetFile(ctx, c.getStructureFilePath(templateName))
	if err != nil {
		return nil, err
	}

	err = uow.Commit()
	if err != nil {
		return nil, err
	}

	return &dto.TemplateInfo{
		TemplateName: templateName,
		Structure:    string(fieldsFile),
	}, nil
}

func (c *GetTemplate) getStructureFilePath(templateName string) string {
	templatePath := filepath.Join(c.cfg.TemplatesFolder, templateName)
	structureFile := filepath.Join(templatePath+c.cfg.PathToFile, c.cfg.Filename)
	return structureFile
}
