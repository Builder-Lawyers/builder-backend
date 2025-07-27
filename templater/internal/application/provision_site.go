package application

import (
	"encoding/json"
	"fmt"
	"github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/build"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/events"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/storage"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type ProvisionSite struct {
	*db.UOWFactory
	*storage.Storage
	*build.TemplateBuild
	buildFolder string
	pathToFile  string
	filename    string
	//*dns.DNSProvisioner
}

func NewProvisionSite(factory *db.UOWFactory, storage *storage.Storage, build *build.TemplateBuild, buildFolder, pathToFile, filename string) *ProvisionSite {
	return &ProvisionSite{
		factory,
		storage,
		build,
		buildFolder,
		pathToFile,
		filename,
	}
}

// download all source code of template, save it to fs
// place page.json to a certain dir
// check if node_modules are installed, install if not
// build to dist folder
// upload dist folder to s3
func (c *ProvisionSite) Handle(event events.SiteAwaitingProvision) error {
	dir, err := os.ReadDir(c.buildFolder)
	if err != nil {
		return err
	}
	if len(dir) == 0 {
		slog.Info("template's directory is empty, downloading sources")
		files := c.Storage.ListFiles()
		err = c.Storage.DownloadFiles(files, c.buildFolder)
		if err != nil {
			slog.Error("error downloading template's sources %v", err)
		}
	}
	err = saveFieldsToFile(event.Fields, c.buildFolder+event.TemplateName+c.pathToFile, c.filename)
	if err != nil {
		slog.Error("error saving fields json to template %v", err)
	}
	slog.Info("Building")
	buildPath, err := c.TemplateBuild.RunFrontendBuild()
	if err != nil {
		return err
	}
	// TODO place templateName here
	if err = c.UploadFiles(event.DomainVariants[0], "", buildPath); err != nil {
		return err
	}

	return nil
}

func (c *ProvisionSite) UploadFiles(approvedDomain, templateName, dir string) error {
	files := readFilesFromDir(dir)
	for _, f := range files {
		file, err := os.Open(f)
		// gets a substring after dist/
		normalized := filepath.ToSlash(f)
		parts := strings.SplitN(normalized, "templates/"+templateName, 2)
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
