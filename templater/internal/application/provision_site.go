package application

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/build"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/certs"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/client/builder"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/config"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/dns"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/events"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/storage"
	"github.com/jackc/pgx/v5"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type ProvisionSite struct {
	cfg *config.ProvisionConfig
	*db.UOWFactory
	*storage.Storage
	*build.TemplateBuild
	*dns.DNSProvisioner
	*certs.ACMCertificates
	*builder.BuilderClient
}

func NewProvisionSite(
	cfg *config.ProvisionConfig, factory *db.UOWFactory, storage *storage.Storage, build *build.TemplateBuild,
	dns *dns.DNSProvisioner, certs *certs.ACMCertificates, builderClient *builder.BuilderClient,
) *ProvisionSite {
	return &ProvisionSite{
		cfg,
		factory,
		storage,
		build,
		dns,
		certs,
		builderClient,
	}
}

// download all source code of template, save it to fs
// place page.json to a certain dir
// check if node_modules are installed, install if not
// build to dist folder
// upload dist folder to s3
func (c *ProvisionSite) Handle(event events.SiteAwaitingProvision) (pgx.Tx, error) {
	dir, err := os.ReadDir(c.cfg.BuildFolder)
	if err != nil {
		return nil, err
	}
	// idempotent execution - check if provisioned site static content already exists
	existingFiles := c.Storage.ListFiles(1, event.DomainVariants[0])
	if len(existingFiles) > 0 {
		return nil, nil
	}
	if len(dir) == 0 {
		slog.Info("template's directory is empty, downloading sources")
		files := c.Storage.ListFiles(1, event.TemplateName)
		err = c.Storage.DownloadFiles(files, c.cfg.BuildFolder)
		if err != nil {
			slog.Error("error downloading template's sources %v", err)
		}
	}
	err = saveFieldsToFile(event.Fields, c.cfg.BuildFolder+event.TemplateName+c.cfg.PathToFile, c.cfg.Filename)
	if err != nil {
		slog.Error("error saving fields json to template %v", err)
	}
	slog.Info("Building")
	buildPath, err := c.TemplateBuild.RunFrontendBuild()
	if err != nil {
		return nil, err
	}
	sitePath := event.DomainVariants[0]
	// TODO place templateName here
	if err = c.UploadFiles(sitePath, "", buildPath); err != nil {
		return nil, err
	}
	certArn, err := c.ACMCertificates.GetARN("")
	if err != nil {
		slog.Error("err getting cert arn", "acm", err)
		return nil, err
	}
	var fullDomain string
	// TODO: if custom domain == customDomain
	fullDomain = fmt.Sprintf("%v.%v", sitePath, c.cfg.BaseDomain)
	distributionID, err := c.DNSProvisioner.MapCfDistributionToS3("/"+sitePath, sitePath, fullDomain, certArn)
	if err != nil {
		slog.Error("err mapping s3 to cloudfront distr", "cf", err)
		return nil, err
	}
	// TODO: based on timings of this, maybe divide the whole process on steps so that long running
	// subtasks won't hold back the whole process
	cfDomain, err := c.DNSProvisioner.WaitAndGetDistribution(context.Background(), distributionID)
	if err != nil {
		slog.Error("err waiting for deployment of distribution", "cf", err)
		return nil, err
	}
	var domainID string
	// TODO: if new domain requested, provision it first
	domainID = c.cfg.DomainID
	err = c.DNSProvisioner.CreateSubdomain(fullDomain, domainID, cfDomain)
	if err != nil {
		slog.Error("err creating route53 subdomain", "r53", err)
		return nil, err
	}

	newStatus := builder.Created
	_, err = c.BuilderClient.UpdateSite(event.SiteID, builder.UpdateSiteRequest{NewStatus: &newStatus})
	if err != nil {
		slog.Error("err updating site's status", "builder", err)
		return nil, err
	}

	return nil, nil
}

func (c *ProvisionSite) UploadFiles(approvedDomain, templateName, dir string) error {
	files := readFilesFromDir(dir)
	for _, f := range files {
		file, err := os.Open(f)
		// gets a substring after dist/
		normalized := filepath.ToSlash(f)
		parts := strings.SplitN(normalized, "template/"+templateName, 2)
		if err != nil {
			return fmt.Errorf("malformed filepath, %s: %v", f, err)
		}
		err = c.Storage.UploadFile(approvedDomain+"/"+parts[1], nil, file)
		if err != nil {
			return fmt.Errorf("can't put object %v", err)
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("failed to close file %s: %v", f, err)
		}
		slog.Info("info", "Uploaded file %v", f)
	}
	return nil
}

func saveFieldsToFile(fields json.RawMessage, relativePath string, filename string) error {
	jsonBytes, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal fields: %w", err)
	}

	if err := os.MkdirAll(relativePath, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", relativePath, err)
	}

	fullPath := filepath.Join(relativePath, filename)
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

func readFilesFromDir(dir string) []string {
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		slog.Error("Can't find provided directory %v", err)
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
