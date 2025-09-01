package mail

import (
	"os"
)

type MailConfig struct {
	SMTPHost string
	SMTPPort string
	Username string
	Password string
}

func NewMailConfig() *MailConfig {
	return &MailConfig{
		SMTPHost: os.Getenv("MAIL_HOST"),
		SMTPPort: os.Getenv("MAIL_PORT"),
		Username: os.Getenv("MAIL_USERNAME"),
		Password: os.Getenv("MAIL_PASSWORD"),
	}
}
