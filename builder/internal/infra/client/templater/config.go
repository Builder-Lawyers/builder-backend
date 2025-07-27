package templater

import (
	"github.com/Builder-Lawyers/builder-backend/pkg/env"
)

type TemplaterConfig struct {
	schema  string
	host    string
	port    string
	version string
}

func NewTemplaterConfig() TemplaterConfig {
	return TemplaterConfig{
		schema:  env.GetEnv("TEMPLATER_SCHEMA", "http://"),
		host:    env.GetEnv("TEMPLATER_HOST", "localhost"),
		port:    env.GetEnv("TEMPLATER_PORT", "3001"),
		version: env.GetEnv("TEMPLATER_V", "/1"),
	}
}
