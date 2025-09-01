package config

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/Builder-Lawyers/builder-backend/pkg/env"
)

type ProvisionConfig struct {
	BuildFolder     string
	TemplatesFolder string
	BucketPath      string
	PathToFile      string
	Filename        string
	BaseDomain      string
	Defaults        *Defaults
}

type Defaults struct {
	S3Domain string
	CertARN  string
}

func NewProvisionConfig() *ProvisionConfig {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	slog.Info("Current working directory: %v", "config", wd)
	parent := filepath.Dir(wd)
	buildFolder := filepath.Join(parent, "templates-repo")
	templatesFolder := filepath.Join(buildFolder, "templates")
	return &ProvisionConfig{
		env.GetEnv("P_BUILD_FOLDER", buildFolder),
		env.GetEnv("P_TEMPLATES_FOLDER", templatesFolder),
		env.GetEnv("P_BUCKET_PATH", "templates-sources/"),
		env.GetEnv("P_PATH_TO_FILE", "/src/pages/"),
		env.GetEnv("P_FILENAME", "_page.json"),
		os.Getenv("P_BASE_DOMAIN"),
		NewDefaults(),
	}
}

func NewDefaults() *Defaults {
	return &Defaults{
		os.Getenv("P_DEFAULT_S3_DOMAIN"),
		os.Getenv("P_DEFAULT_CERT_ARN"),
	}
}
