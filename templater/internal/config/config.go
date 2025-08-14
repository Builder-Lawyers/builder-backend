package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Builder-Lawyers/builder-backend/pkg/env"
)

type ProvisionConfig struct {
	BuildFolder string
	PathToFile  string
	Filename    string
	BaseDomain  string
	Defaults    *Defaults
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
	fmt.Println("Current working directory:", wd)
	parent := filepath.Dir(filepath.Dir(wd))
	buildFolder := filepath.Join(parent, "templates-monorepo", "templates")
	//target := filepath.Join(parent, "templates")
	fmt.Println(buildFolder)
	return &ProvisionConfig{
		env.GetEnv("P_BUILD_FOLDER", buildFolder),
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
