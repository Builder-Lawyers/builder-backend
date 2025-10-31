package template

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/Builder-Lawyers/builder-backend/internal/application/dto"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/build"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/config"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/dns"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/storage"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type UpdateTemplate struct {
	uowFactory     *dbs.UOWFactory
	storage        *storage.Storage
	templateBuild  *build.TemplateBuild
	dnsProvisioner *dns.DNSProvisioner
	cfg            config.ProvisionConfig
}

func NewUpdateTemplate(uowFactory *dbs.UOWFactory, storage *storage.Storage,
	templateBuild *build.TemplateBuild, dnsProvisioner *dns.DNSProvisioner, cfg config.ProvisionConfig,
) *UpdateTemplate {
	return &UpdateTemplate{uowFactory: uowFactory, storage: storage, templateBuild: templateBuild, dnsProvisioner: dnsProvisioner, cfg: cfg}
}

// Refreshes all local template files, rebuilds a template and uploads built statics to s3
func (c *UpdateTemplate) Execute(ctx context.Context, req *dto.UpdateTemplatesRequest) error {
	// TODO: guard against unauthorized access
	var err error
	templatesToUpdate := make([]string, 0, 1)
	if req.Name == nil {
		templatesToUpdate, err = c.getAllTemplates(ctx)
		if err != nil {
			return err
		}
	} else {
		exists, err := c.isTemplateValid(ctx, *req.Name)
		if err != nil || !exists {
			return fmt.Errorf("requested template doesn't exists, %v", *req.Name)
		}
		templatesToUpdate = append(templatesToUpdate, *req.Name)
	}

	templateStylesURLs := make(map[string]db.Template, len(templatesToUpdate))
	for _, template := range templatesToUpdate {
		//bucketPath := fmt.Sprintf("%s%s/%s", c.cfg.TemplateSrcBucketPath, "templates", template)
		localPath := filepath.Join(c.cfg.TemplatesFolder, template)
		err = c.templateBuild.ClearTemplateFilesLocally(localPath)
		if err != nil {
			return fmt.Errorf("err clearing old template sources, %v", err)
		}
		err = c.templateBuild.RefreshTemplate(ctx, template)
		if err != nil {
			return err
		}
		buildOutputDir, err := c.templateBuild.RunSiteBuild(ctx, localPath)
		if err != nil {
			return fmt.Errorf("err building template, %v", err)
		}

		templateBuildS3Path := fmt.Sprintf("%s%s", c.cfg.TemplateBuildBucketPath, template)
		if err = c.templateBuild.UploadFiles(ctx, templateBuildS3Path, template, buildOutputDir); err != nil {
			return fmt.Errorf("err saving build output to s3, %v", err)
		}

		domain := fmt.Sprintf("%v.%v", template, c.cfg.BaseDomain)
		previewURL, err := c.dnsProvisioner.MapCfDistributionToS3GetURL(ctx, "/"+templateBuildS3Path, c.cfg.Defaults.S3Domain, domain, c.cfg.Defaults.CertARN)
		if err != nil {
			return err
		}
		styles := c.storage.ListFiles(ctx, 1, &s3.ListObjectsV2Input{
			Prefix: aws.String(templateBuildS3Path),
		})
		if styles == nil || len(styles) == 0 {
			return fmt.Errorf("err getting styles file from template, %v", err)
		}
		stylesPath := fmt.Sprintf("%s/%s", c.cfg.S3ObjectURL, styles[0])

		templateStylesURLs[template] = db.Template{
			Styles:  stylesPath,
			Preview: previewURL,
		}
	}

	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return err
	}
	defer uow.Finalize(&err)
	for name, template := range templateStylesURLs {
		_, err = tx.Exec(ctx, "UPDATE builder.templates SET styles = $1, preview = $2 WHERE name = $3",
			template.Styles, template.Preview, name)
		if err != nil {
			return fmt.Errorf("err inserting styles url to template, %v", err)
		}
	}

	return nil
}

func (c *UpdateTemplate) isTemplateValid(ctx context.Context, templateName string) (bool, error) {
	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return false, err
	}
	defer uow.Finalize(&err)
	var templateID sql.NullInt64
	err = tx.QueryRow(ctx, "SELECT id FROM builder.templates WHERE name = $1", templateName).Scan(&templateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("err getting template name %v", err)
	}

	return templateID.Valid, nil
}

func (c *UpdateTemplate) getAllTemplates(ctx context.Context) ([]string, error) {
	templatesToUpdate := make([]string, 0, 1)
	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return nil, err
	}
	defer uow.Finalize(&err)
	rows, err := tx.Query(ctx, "SELECT name FROM builder.templates")
	if err != nil {
		return nil, fmt.Errorf("err getting templates %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var templateName string
		if err = rows.Scan(&templateName); err != nil {
			return nil, err
		}
		templatesToUpdate = append(templatesToUpdate, templateName)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return templatesToUpdate, nil
}
