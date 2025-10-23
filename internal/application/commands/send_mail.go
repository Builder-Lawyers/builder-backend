package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"strings"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application/events"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/mail"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	shared "github.com/Builder-Lawyers/builder-backend/pkg/interfaces"
)

type SendMail struct {
	server     *mail.MailServer
	uowFactory *dbs.UOWFactory
}

func NewSendMail(server *mail.MailServer, uowFactory *dbs.UOWFactory) *SendMail {
	return &SendMail{server: server, uowFactory: uowFactory}
}

func (c *SendMail) Handle(ctx context.Context, event events.SendMail) (shared.UoW, error) {
	mailData, err := mapToMailData(event)
	if err != nil {
		return nil, err
	}
	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return nil, err
	}
	var email string
	err = tx.QueryRow(ctx, "SELECT email FROM builder.users WHERE id = $1", event.UserID).Scan(&email)
	if err != nil {
		return nil, err
	}
	recipients := make([]string, 0)
	recipients = append(recipients, email)

	var mailTemplate string
	err = tx.QueryRow(ctx, "SELECT content FROM builder.mail_templates WHERE type = $1", mailData.GetMailType()).Scan(&mailTemplate)
	if err != nil {
		return uow, err
	}

	htmlBody, err := renderHTML(mailTemplate, mailData)
	if err != nil {
		return uow, fmt.Errorf("error rendering html, %v", err)
	}

	mail := db.Mail{
		MailType:   mailData.GetMailType(),
		Recipients: strings.Join(recipients, ","),
		Subject:    event.Subject,
		Content:    htmlBody,
		SentAt:     time.Now(),
	}
	_, err = tx.Exec(ctx, "INSERT INTO builder.mails(type, recipients, subject, content, sent_at) VALUES ($1,$2,$3,$4,$5)",
		mail.MailType, mail.Recipients, mail.Subject, mail.Content, mail.SentAt,
	)
	if err != nil {
		return uow, err
	}
	err = c.server.SendMail(recipients, mail.Subject, mail.Content)
	if err != nil {
		return uow, err
	}

	slog.Info("mail sent", "id", mail.ID)

	return uow, nil
}

func renderHTML(tmpl string, data mail.MailData) (string, error) {
	t := template.Must(template.New("email").Parse(tmpl))
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func mapToMailData(event events.SendMail) (mail.MailData, error) {
	factory, ok := mailDataRegistry[event.Subject]
	if !ok {
		return nil, fmt.Errorf("no such mailData type for subject: %s", event.Subject)
	}

	instance := factory()

	raw, err := json.Marshal(event.Data)
	if err != nil {
		return nil, fmt.Errorf("marshal error: %w", err)
	}

	if err := json.Unmarshal(raw, instance); err != nil {
		return nil, fmt.Errorf("unmarshal error: %w", err)
	}

	return instance, nil
}

var mailDataRegistry = map[string]func() mail.MailData{
	mail.FreeTrialEndsData{}.GetSubject():       func() mail.MailData { return &mail.FreeTrialEndsData{} },
	mail.SiteCreatedData{}.GetSubject():         func() mail.MailData { return &mail.SiteCreatedData{} },
	mail.SiteDeactivatedData{}.GetSubject():     func() mail.MailData { return &mail.SiteDeactivatedData{} },
	mail.RegistrationConfirmData{}.GetSubject(): func() mail.MailData { return &mail.RegistrationConfirmData{} },
}
