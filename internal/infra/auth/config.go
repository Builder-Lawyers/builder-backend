package auth

import (
	"log/slog"
	"os"

	"github.com/google/uuid"
)

type OIDCConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	IssuerURL    string
	Mode         string
	TestUser     *uuid.UUID
}

func NewOIDCConfig() *OIDCConfig {
	var testUserID uuid.UUID
	testUser := os.Getenv("TEST_USER")
	if testUser != "" {
		var err error
		testUserID, err = uuid.Parse(testUser)
		if err != nil {
			slog.Error("error getting test user ID", "err", err)
			return nil
		}
	}
	return &OIDCConfig{
		ClientID:     os.Getenv("COGNITO_ID"),
		ClientSecret: os.Getenv("COGNITO_SECRET"),
		RedirectURL:  os.Getenv("COGNITO_REDIRECT"),
		IssuerURL:    os.Getenv("COGNITO_ISSUER"),
		Mode:         os.Getenv("MODE"),
		TestUser:     &testUserID,
	}
}
