package processors

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application/consts"
	"github.com/Builder-Lawyers/builder-backend/internal/application/events"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/build"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/certs"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/config"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db/repo"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/dns"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/storage"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	shared "github.com/Builder-Lawyers/builder-backend/pkg/interfaces"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type ProvisionSite struct {
	cfg            config.ProvisionConfig
	uowFactory     *dbs.UOWFactory
	storage        *storage.Storage
	templateBuild  *build.TemplateBuild
	dnsProvisioner *dns.DNSProvisioner
	certs          *certs.ACMCertificates
}

func NewProvisionSite(
	cfg config.ProvisionConfig, factory *dbs.UOWFactory, storage *storage.Storage,
	build *build.TemplateBuild, dns *dns.DNSProvisioner, certs *certs.ACMCertificates,
) *ProvisionSite {
	return &ProvisionSite{
		cfg,
		factory,
		storage,
		build,
		dns,
		certs,
	}
}

// download all source code of template, save it to fs
// place page.json to a certain dir
// check if node_modules are installed, install if not
// build to dist folder
// upload dist folder to s3
func (c *ProvisionSite) Handle(ctx context.Context, event events.SiteAwaitingProvision) (shared.UoW, error) {
	siteID := strconv.FormatUint(event.SiteID, 10)
	err := c.templateBuild.DownloadTemplate(ctx, event.TemplateName)
	if err != nil {
		return nil, err
	}
	// idempotent execution - check if provisioned site static content already exists
	existingFiles := c.storage.ListFiles(ctx, 1, &s3.ListObjectsV2Input{
		Prefix: aws.String(siteID),
	})
	if len(existingFiles) > 0 {
		return nil, nil
	}
	templatePath := filepath.Join(c.cfg.TemplatesFolder, event.TemplateName)
	customizeJsonPath := filepath.Join(templatePath+c.cfg.PathToFile, c.cfg.Filename)
	err = saveFieldsToFile(event.Fields, customizeJsonPath)
	if err != nil {
		slog.Error("error saving fields json to template", "build", err)
		return nil, err
	}

	slog.Info("Building")
	buildPath, err := c.templateBuild.RunSiteBuild(ctx, templatePath)
	if err != nil {
		return nil, err
	}
	cleanBuild(customizeJsonPath)

	if err = c.templateBuild.UploadFiles(ctx, "sites/"+siteID, event.TemplateName, buildPath); err != nil {
		return nil, err
	}

	var domain string
	var newEvent shared.Event
	var newProvision db.Provision

	switch event.DomainType {
	case consts.DefaultDomain:

		domain = fmt.Sprintf("%v.%v", event.Domain, c.cfg.BaseDomain)
		timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		distributionID, err := c.dnsProvisioner.MapCfDistributionToS3GetID(timeoutCtx, "/sites/"+siteID, c.cfg.Defaults.S3Domain, domain, c.cfg.Defaults.CertARN)
		cancel()
		if err != nil {
			return nil, err
		}
		newEvent = events.FinalizeProvision{
			SiteID:         event.SiteID,
			DistributionID: distributionID,
			Domain:         domain,
			DomainType:     event.DomainType,
			CreatedAt:      time.Now(),
		}
		newProvision = db.Provision{
			SiteID:         event.SiteID,
			Type:           event.DomainType,
			Status:         consts.ProvisionStatusInProcess,
			Domain:         domain,
			CertificateARN: c.cfg.Defaults.CertARN,
			CloudfrontID:   distributionID,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		break

	case consts.SeparateDomain:

		domain = event.Domain
		timeoutCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		operationID, err := c.dnsProvisioner.RequestDomain(timeoutCtx, domain)
		cancel()
		if err != nil {
			return nil, err
		}
		// for now is a FQDN, maybe do with asterisk like a *.baseDomain?
		certificateARN, err := c.certs.CreateCertificate(ctx, domain)
		if err != nil {
			return nil, err
		}

		newEvent = events.ProvisionCDN{
			SiteID:         event.SiteID,
			OperationID:    operationID,
			CertificateARN: certificateARN,
			Domain:         domain,
			CreatedAt:      time.Now(),
		}

		newProvision = db.Provision{
			SiteID:         event.SiteID,
			Type:           event.DomainType,
			Status:         consts.ProvisionStatusInProcess,
			Domain:         domain,
			CertificateARN: certificateARN,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		break
	default:
		return nil, fmt.Errorf("unknown domain type")
	}

	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return nil, err
	}

	eventRepo := repo.NewEventRepo(tx)
	err = eventRepo.InsertEvent(ctx, newEvent)
	if err != nil {
		return uow, err
	}
	provisionRepo := repo.NewProvisionRepo(tx)
	err = provisionRepo.InsertProvision(ctx, newProvision)
	if err != nil {
		return uow, err
	}

	return uow, nil
}

func saveFieldsToFile(fields json.RawMessage, fullPath string) error {
	jsonBytes, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal fields: %w", err)
	}
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", fullPath, err)
	}
	defer file.Close()

	if _, err := file.Write(jsonBytes); err != nil {
		return fmt.Errorf("failed to write JSON to file %s: %w", fullPath, err)
	}

	return nil
}

func cleanBuild(filename string) {
	err := os.Remove(filename)
	if err != nil {
		slog.Error("error cleaning up", "cleanup", err)
		return
	}
	slog.Info("Success build cleanup", "cleanup", "")
}
