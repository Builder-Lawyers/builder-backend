package template

import (
	"context"
	"fmt"

	"github.com/Builder-Lawyers/builder-backend/internal/application/dto"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
)

type UpdateTemplate struct {
	uowFactory *dbs.UOWFactory
}

func NewUpdateTemplate(uowFactory *dbs.UOWFactory,
) *UpdateTemplate {
	return &UpdateTemplate{uowFactory: uowFactory}
}

// Refreshes all local template files, rebuilds a template and uploads built statics to s3
func (c *UpdateTemplate) Execute(ctx context.Context, id int, req *dto.UpdateTemplateRequest) error {
	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return err
	}
	defer uow.Finalize(&err)
	_, err = tx.Exec(ctx, "UPDATE builder.templates SET file_id = COALESCE($1, file_id), name= COALESCE($2, name) WHERE id = $3 ", req.FileID, req.Name, id)
	if err != nil {
		return fmt.Errorf("err executing a partial update, %w", err)
	}
	return nil
}
