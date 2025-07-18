package db

import (
	"fmt"
	"github.com/Builder-Lawyers/builder-backend/env"
	"strconv"
)

type Config struct {
	Host     string
	Port     int
	Database string
	User     string
	Password string
}

func NewConfig() Config {
	port, err := strconv.Atoi(env.GetEnv("PG_PORT", "5432"))
	if err != nil {
		panic(err)
	}
	return Config{
		Host:     env.GetEnv("PG_HOST", "localhost"),
		Port:     port,
		Database: env.GetEnv("PG_DB", "postgres"),
		User:     env.GetEnv("PG_USER", "postgres"),
		Password: env.GetEnv("PG_PASSWORD", "postgres"),
	}
}
func (conf *Config) GetDSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s",
		conf.User, conf.Password, conf.Host, conf.Port, conf.Database,
	)
}
