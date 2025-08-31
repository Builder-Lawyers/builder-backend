package builder

import (
	"github.com/Builder-Lawyers/builder-backend/pkg/env"
)

type BuilderConfig struct {
	schema  string
	host    string
	port    string
	version string
}

func NewBuilderConfig() *BuilderConfig {
	return &BuilderConfig{
		schema:  env.GetEnv("BUILDER_SCHEMA", "http"),
		host:    env.GetEnv("BUILDER_HOST", "localhost"),
		port:    env.GetEnv("BUILDER_PORT", "3001"),
		version: env.GetEnv("BUILDER_V", "/1"),
	}
}
