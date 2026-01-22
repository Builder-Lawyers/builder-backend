package site

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application/consts"
	"github.com/Builder-Lawyers/builder-backend/internal/application/dto"
	"github.com/Builder-Lawyers/builder-backend/internal/application/events"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/auth"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/build"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/config"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db/repo"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/dns"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/storage"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/aws/aws-sdk-go-v2/aws"
)

type UpdateSite struct {
	uowFactory     *dbs.UOWFactory
	templateBuild  *build.TemplateBuild
	dnsProvisioner *dns.DNSProvisioner
	storage        *storage.Storage
	cfg            config.ProvisionConfig
}

func NewUpdateSite(factory *dbs.UOWFactory, templateBuild *build.TemplateBuild, dns *dns.DNSProvisioner, storage *storage.Storage, cfg config.ProvisionConfig) *UpdateSite {
	return &UpdateSite{uowFactory: factory, templateBuild: templateBuild, dnsProvisioner: dns, storage: storage, cfg: cfg}
}

func (c *UpdateSite) Execute(ctx context.Context, siteID uint64, req *dto.UpdateSiteRequest, identity *auth.Identity) (uint64, error) {
	var site db.Site
	var domainType consts.ProvisionType

	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return 0, err
	}
	defer uow.Finalize(&err)

	err = tx.QueryRow(ctx, "SELECT creator_id, template_id, status, fields from builder.sites WHERE id = $1", siteID).Scan(
		&site.CreatorID,
		&site.TemplateID,
		&site.Status,
		&site.Fields,
	)
	if err != nil {
		return 0, err
	}

	//if identity.UserID != site.CreatorID {
	//	return 0, errs.PermissionsError{Err: fmt.Errorf("user requesting action, is not a site's creator")}
	//}

	if req.NewStatus != nil {
		// SiteStatusAwaitingProvision - from frontend, all fields are filled in by user
		site.Status = consts.SiteStatus(*req.NewStatus)
		_, err = tx.Exec(ctx, "UPDATE builder.sites SET status = $1, updated_at = $2 WHERE id = $3", site.Status, time.Now(), siteID)
		if err != nil {
			return 0, err
		}

		switch site.Status {
		case consts.SiteStatusAwaitingProvision:
			var siteProvisioned int
			err = tx.QueryRow(ctx, "select count(*) from builder.provisions where site_id = $1", site.ID).Scan(&siteProvisioned)
			if err != nil {
				return 0, fmt.Errorf("err checking if site already provisioned, %v", err)
			}
			if siteProvisioned > 0 {
				slog.Warn("site already provisioned", "id", siteID)
				return siteID, nil
			}
			slog.Info("requesting site provision", "siteID", siteID)
			var templateName string
			err = tx.QueryRow(ctx, "SELECT name FROM builder.templates WHERE id = $1", site.TemplateID).Scan(&templateName)
			if err != nil {
				return 0, err
			}
			fields := db.RawMessageToMap(site.Fields)
			if req.Fields != nil {
				fields = *req.Fields
			}
			if req.DomainType == nil {
				domainType = consts.DefaultDomain
			} else {
				domainType = consts.ProvisionType(*req.DomainType)
			}
			provisionReq := dto.ProvisionSiteRequest{
				SiteID:       siteID,
				DomainType:   domainType,
				TemplateName: templateName,
				Domain:       *req.Domain,
				Fields:       fields,
			}
			payload, err := json.Marshal(provisionReq)
			if err != nil {
				return 0, err
			}
			outbox := db.Outbox{
				Event:     "SiteAwaitingProvision",
				Status:    int(consts.NotProcessed),
				Payload:   json.RawMessage(payload),
				CreatedAt: time.Now(),
			}
			err = tx.QueryRow(ctx, "INSERT INTO builder.outbox (event, status, payload, created_at) VALUES ($1,$2,$3,$4) RETURNING id",
				outbox.Event, outbox.Status, outbox.Payload, outbox.CreatedAt).Scan(&outbox.ID)
			if err != nil {
				return 0, err
			}
		case consts.SiteStatusAwaitingDeactivation:

			deactivateSiteEvent := events.DeactivateSite{
				SiteID: siteID,
				Reason: "Deactivated due to missing payment",
			}

			eventRepo := repo.NewEventRepo(tx)
			err = eventRepo.InsertEvent(ctx, deactivateSiteEvent)
			if err != nil {
				return 0, err
			}
		}
		return siteID, nil

	}

	_, err = tx.Exec(ctx, "UPDATE builder.sites SET fields = COALESCE($1, fields), file_id = COALESCE($2, file_id), updated_at = $3 WHERE id = $4",
		*req.Fields, req.FileID, time.Now(), siteID)
	if err != nil {
		return 0, err
	}

	// if pages.json is changed -> rebuild site, update pages.json on s3

	if req.Fields != nil {
		sitePath := "sites/" + strconv.FormatUint(siteID, 10)
		var templateName string
		err = tx.QueryRow(ctx, "SELECT name FROM builder.templates WHERE id = $1", site.TemplateID).Scan(&templateName)
		if err != nil {
			return 0, fmt.Errorf("err getting template's name, %v", err)
		}

		// download template sources if needed
		err = c.templateBuild.DownloadTemplate(ctx, templateName)
		if err != nil {
			return 0, err
		}
		// upload pages.json structure file
		templatePath := filepath.Join(c.cfg.TemplatesFolder, templateName)
		customizeJsonPath := filepath.Join(templatePath+c.cfg.PathToFile, c.cfg.Filename)
		err = saveFieldsToFile(*req.Fields, customizeJsonPath)
		if err != nil {
			return 0, err
		}
		structureFile, err := os.Open(customizeJsonPath)
		if err != nil {
			return 0, fmt.Errorf("err reading site structure file, %v", err)
		}
		defer structureFile.Close()
		structureFile.Sync()
		_, err = c.storage.UploadFile(ctx, sitePath+"/"+c.cfg.Filename, aws.String("application/json"), structureFile)
		if err != nil {
			return 0, fmt.Errorf("err uploading structure file, %v", err)
		}

		// rebuild and upload site
		err = c.buildSite(ctx, sitePath, templatePath, templateName)
		if err != nil {
			return 0, fmt.Errorf("err building site, %v", err)
		}
		provisionRepo := repo.NewProvisionRepo(tx)
		provision, err := provisionRepo.GetProvisionByID(ctx, siteID)
		if err != nil {
			return 0, fmt.Errorf("err getting provision, %v", err)
		}
		err = c.dnsProvisioner.InvalidateDistribution(ctx, provision.CloudfrontID)
		if err != nil {
			return 0, fmt.Errorf("err invalidating cf distribution, %v", err)
		}

	}

	return siteID, nil
}

func (c *UpdateSite) buildSite(ctx context.Context, sitePath, templatePath, templateName string) error {
	slog.Info("Building")
	time.Sleep(2 * time.Second)
	buildPath, err := c.templateBuild.RunSiteBuild(ctx, templatePath)
	if err != nil {
		return err
	}

	return c.templateBuild.UploadFiles(ctx, sitePath, templateName, buildPath)
}

func saveFieldsToFile(fields []map[string]interface{}, path string) error {
	jsonBytes, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	slog.Info("saving fields to file", "fields", string(jsonBytes), "path", path)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, jsonBytes, 0o644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	return true
}
