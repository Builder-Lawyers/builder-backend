package config

import (
	"os"
	"path/filepath"

	"github.com/Builder-Lawyers/builder-backend/pkg/env"
)

type ProvisionConfig struct {
	BuildFolder     string
	TemplatesFolder string
	S3ObjectURL     string
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

func NewProvisionConfig() ProvisionConfig {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	parent := filepath.Dir(wd)
	buildFolder := filepath.Join(parent, "templates-repo")
	templatesFolder := filepath.Join(buildFolder, "templates")
	return ProvisionConfig{
		BuildFolder:     env.GetEnv("P_BUILD_FOLDER", buildFolder),
		TemplatesFolder: env.GetEnv("P_TEMPLATES_FOLDER", templatesFolder),
		S3ObjectURL:     os.Getenv("P_S3_OBJECT_URL"),
		BucketPath:      env.GetEnv("P_BUCKET_PATH", "templates-sources/"),
		PathToFile:      env.GetEnv("P_PATH_TO_FILE", "/"),
		Filename:        env.GetEnv("P_FILENAME", "pages.json"),
		BaseDomain:      os.Getenv("P_BASE_DOMAIN"),
		Defaults:        NewDefaults(),
	}
}

func NewDefaults() *Defaults {
	return &Defaults{
		os.Getenv("P_DEFAULT_S3_DOMAIN"),
		os.Getenv("P_DEFAULT_CERT_ARN"),
	}
}
