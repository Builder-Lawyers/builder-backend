package mail

import (
	"fmt"
	"net/smtp"
	"strings"
)

type MailServer struct {
	cfg  *MailConfig
	auth smtp.Auth
}

func NewMailServer(cfg *MailConfig) *MailServer {
	return &MailServer{
		cfg:  cfg,
		auth: smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost),
	}
}

func (m *MailServer) SendMail(to []string, subject, body string) error {
	addr := m.cfg.SMTPHost + ":" + m.cfg.SMTPPort

	headers := make(map[string]string)
	headers["From"] = m.cfg.Username
	headers["To"] = strings.Join(to, ",")
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/plain; charset=\"utf-8\""

	// Build the full message
	var msg strings.Builder
	for k, v := range headers {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}

	msg.WriteString("\r\n" + body)
	err := smtp.SendMail(addr, m.auth, m.cfg.Username, to, []byte(msg.String()))
	if err != nil {
		return fmt.Errorf("failed to send mail: %w", err)
	}
	return nil
}
