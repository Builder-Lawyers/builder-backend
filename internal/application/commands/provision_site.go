package commands

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
	"strings"
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
	cfg            *config.ProvisionConfig
	uowFactory     *dbs.UOWFactory
	storage        *storage.Storage
	templateBuild  *build.TemplateBuild
	dnsProvisioner *dns.DNSProvisioner
	certs          *certs.ACMCertificates
}

func NewProvisionSite(
	cfg *config.ProvisionConfig, factory *dbs.UOWFactory, storage *storage.Storage,
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
	err := c.downloadTemplate(ctx, event.TemplateName)
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
	buildPath, err := c.templateBuild.RunFrontendBuild(ctx, templatePath)
	if err != nil {
		return nil, err
	}
	cleanBuild(customizeJsonPath)

	if err = c.uploadFiles(ctx, "sites/"+siteID, event.TemplateName, buildPath); err != nil {
		return nil, err
	}

	var domain string
	var newEvent shared.Event
	var newProvision db.Provision

	switch event.DomainType {
	case consts.DefaultDomain:

		domain = fmt.Sprintf("%v.%v", event.Domain, c.cfg.BaseDomain)
		timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		distributionID, err := c.dnsProvisioner.MapCfDistributionToS3(timeoutCtx, "/sites/"+siteID, c.cfg.Defaults.S3Domain, domain, c.cfg.Defaults.CertARN)
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

// Uploads file from a filesystem path to a s3 prefix
func (c *ProvisionSite) uploadFiles(ctx context.Context, siteID, templateName, dir string) error {
	files := readFilesFromDir(dir)
	for _, f := range files {
		file, err := os.Open(f)
		// gets a substring after dist/
		normalized := filepath.ToSlash(f)
		parts := strings.SplitN(normalized, templateName+"/dist", 2)
		//parts := strings.SplitN(normalized, "template/"+templateName, 2)
		if err != nil {
			return fmt.Errorf("malformed filepath, %s: %v", f, err)
		}
		_, err = c.storage.UploadFile(ctx, siteID+parts[1], nil, file)
		if err != nil {
			return fmt.Errorf("can't put object %v", err)
		}
		if err = file.Close(); err != nil {
			return fmt.Errorf("failed to close file %s: %v", f, err)
		}
		slog.Info("Uploaded file", "fileUpload", f)
	}
	return nil
}

func (c *ProvisionSite) downloadTemplate(ctx context.Context, templateName string) error {
	// can it download templates from s3?
	targetTemplate := filepath.Join(c.cfg.TemplatesFolder, templateName)
	exists, err := dirExists(targetTemplate)
	if err != nil {
		return err
	}
	if exists {
		dir, err := os.ReadDir(targetTemplate)
		if err != nil {
			return err
		}
		if len(dir) > 0 {
			return nil
		}
	}
	slog.Warn("Folder with templates doesn't exist, creating now")
	err = os.MkdirAll(filepath.Join(c.cfg.TemplatesFolder, templateName), fs.ModeDir)
	if err != nil {
		slog.Error("Failed to create dirs for templates", "template", err)
		return err
	}
	slog.Info("Created folders for templates")
	err = c.downloadMissingRootFiles(ctx, c.cfg.BuildFolder, c.cfg.BucketPath)
	if err != nil {
		return err
	}
	bucketPath := fmt.Sprintf("%v%v/%v", c.cfg.BucketPath, "templates", templateName)
	err = c.downloadMissingTemplateFiles(ctx, targetTemplate, bucketPath)
	if err != nil {
		return err
	}
	//dir, err := os.ReadDir(c.cfg.BuildFolder)
	//if err != nil {
	//	return err
	//}
	//if len(dir) == 0 {
	//	slog.Info("template's directory is empty, downloading sources")
	//	files := c.Storage.ListFiles(1, templateName)
	//	err = c.Storage.DownloadFiles(files, c.cfg.BuildFolder)
	//	if err != nil {
	//		slog.Error("error downloading template's sources %v", "template", err)
	//	}
	//}
	return nil
}

func (c *ProvisionSite) downloadMissingRootFiles(ctx context.Context, path, bucketPath string) error {
	dir, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(dir) == 1 { // if only templates folder is present
		slog.Info("directory %v is empty, downloading sources", "downloadTemplate", path)
		files := c.storage.ListFiles(ctx, 100, &s3.ListObjectsV2Input{
			Prefix: aws.String(bucketPath),
		})
		filesToDownload := make([]string, 0)
		for _, file := range files {
			// everything under templates/
			if strings.Contains(file, "/templates/") || file == bucketPath {
				continue
			}
			filesToDownload = append(filesToDownload, file)
		}
		err = c.storage.DownloadFiles(ctx, filesToDownload, path, bucketPath)
		if err != nil {
			slog.Error("error downloading template's sources %v", "template", err)
			return err
		}
	}

	return nil
}

func (c *ProvisionSite) downloadMissingTemplateFiles(ctx context.Context, path, bucketPath string) error {
	dir, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(dir) == 0 {
		slog.Info("directory is empty, downloading sources", "dir", path)
		files := c.storage.ListFiles(ctx, 100, &s3.ListObjectsV2Input{
			Prefix: aws.String(bucketPath),
		})
		err = c.storage.DownloadFiles(ctx, files, path, bucketPath)
		if err != nil {
			slog.Error("error downloading template's sources %v", "template", err)
			return err
		}
	}

	return nil
}

func dirExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
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

func readFilesFromDir(dir string) []string {
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		slog.Error("Can't find provided directory", "dir", err)
	}
	for _, entry := range entries {
		fullPath := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			subFiles := readFilesFromDir(fullPath)
			files = append(files, subFiles...)
		} else {
			files = append(files, fullPath)
		}
	}
	return files
}
