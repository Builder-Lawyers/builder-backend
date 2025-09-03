package auth

import "os"

type OIDCConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	IssuerURL    string
}

func NewOIDCConfig() *OIDCConfig {
	return &OIDCConfig{
		ClientID:     os.Getenv("COGNITO_ID"),
		ClientSecret: os.Getenv("COGNITO_SECRET"),
		RedirectURL:  os.Getenv("COGNITO_REDIRECT"),
		IssuerURL:    os.Getenv("COGNITO_ISSUER"),
	}
}
