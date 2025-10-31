package query

import (
	"context"
	"database/sql"
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
	cfg        config.ProvisionConfig
}

func NewGetTemplate(uowFactory *db.UOWFactory, storage *storage.Storage, provisionConfig config.ProvisionConfig) *GetTemplate {
	return &GetTemplate{uowFactory: uowFactory, storage: storage, cfg: provisionConfig}
}

func (c *GetTemplate) Query(ctx context.Context, templateID uint16) (*dto.TemplateInfo, error) {

	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return nil, err
	}
	defer uow.Finalize(&err)

	var templateName string
	var styles sql.NullString
	var preview sql.NullString
	err = tx.QueryRow(ctx, "SELECT name, styles, preview FROM builder.templates WHERE id = $1", templateID).
		Scan(&templateName, &styles, &preview)
	if err != nil {
		return nil, fmt.Errorf("err getting template name")
	}

	templateInfo := &dto.TemplateInfo{
		Id:           int(templateID),
		TemplateName: templateName,
		Styles:       styles.String,
	}

	if styles.Valid {
		templateInfo.Structure = c.getStructureFilePath(templateName)
	}
	if preview.Valid {
		templateInfo.Preview = preview.String
	}

	return templateInfo, nil
}

func (c *GetTemplate) QueryList(ctx context.Context, req *dto.ListTemplatePaginator) (*dto.ListTemplateInfo, error) {
	var err error
	page := 0
	size := 5
	if req.Page != nil {
		page = *req.Page
	}
	if req.Size != nil {
		size = *req.Size
	}
	offset := page * size

	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return nil, err
	}
	defer uow.Finalize(&err)

	rows, err := tx.Query(
		ctx,
		`SELECT id, name, styles, preview  
		 FROM builder.templates 
		 ORDER BY id ASC 
		 LIMIT $1 OFFSET $2`,
		size, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]dto.TemplateInfo, 0, size)
	for rows.Next() {
		var t dto.TemplateInfo
		var styles sql.NullString
		if err := rows.Scan(&t.Id, &t.TemplateName, &styles, &t.Preview); err != nil {
			return nil, err
		}
		t.Styles = styles.String
		// if styles are present == template is built
		if styles.Valid {
			t.Structure = c.getStructureFilePath(t.TemplateName)
		}
		list = append(list, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var total int
	err = tx.QueryRow(ctx, `SELECT count(*) FROM builder.templates`).Scan(&total)
	if err != nil {
		return nil, err
	}

	return &dto.ListTemplateInfo{
		Elements: list,
		Total:    total,
		Page:     page,
	}, nil
}

func (c *GetTemplate) getStructureFilePath(templateName string) string {
	templatePath := fmt.Sprintf("%stemplates/%s", c.cfg.TemplateSrcBucketPath, templateName)
	structureFile := fmt.Sprintf("%s/%s%s", templatePath, c.cfg.PathToFile, c.cfg.Filename)
	return fmt.Sprintf("%v/%v", c.cfg.S3ObjectURL, structureFile)
}

func (c *GetTemplate) getStylesFilePath(templateName string) string {
	templatePath := filepath.Join(c.cfg.TemplateSrcBucketPath, "templates", templateName)
	structureFile := filepath.Join(templatePath+c.cfg.PathToFile, c.cfg.Filename)
	return fmt.Sprintf("%s/%s", c.cfg.S3ObjectURL, structureFile)
}
