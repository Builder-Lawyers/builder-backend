package auth

import (
	"log/slog"
	"os"
	"strconv"

	"github.com/Builder-Lawyers/builder-backend/pkg/env"
	"github.com/google/uuid"
)

type OIDCConfig struct {
	UserPoolID                 string
	RedirectURL                string
	ConfirmationExpirationMins int
	IssuerURL                  string
	GoogleIssuerURL            string
	Mode                       string
	TestUser                   *uuid.UUID
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
	confirmationExpMin, err := strconv.Atoi(env.GetEnv("SIGNUP_CONFIRM_EXP", "60"))
	if err != nil {
		slog.Error("err parsing SIGNUP_CONFIRM_EXP, set to default", "err", err)
		confirmationExpMin = 60
	}
	return &OIDCConfig{
		UserPoolID:                 os.Getenv("COGNITO_POOL_ID"),
		RedirectURL:                os.Getenv("SIGNUP_REDIRECT"),
		ConfirmationExpirationMins: confirmationExpMin,
		IssuerURL:                  os.Getenv("COGNITO_ISSUER"),
		GoogleIssuerURL:            os.Getenv("GOOGLE_ISSUER"),
		Mode:                       os.Getenv("MODE"),
		TestUser:                   &testUserID,
	}
}
