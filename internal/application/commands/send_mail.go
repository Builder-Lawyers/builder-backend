package commands

import (
	"bytes"
	"context"
	"html/template"
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

func (c *SendMail) Handle(event events.SendMail) (shared.UoW, error) {
	mailData := event.Data.(mail.MailData)
	uow := c.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		return nil, err
	}
	var email string
	err = tx.QueryRow(context.Background(), "SELECT email FROM builder.users WHERE id = $1", event.UserID).Scan(&email)
	if err != nil {
		return nil, err
	}
	recipients := make([]string, 0)
	recipients = append(recipients, email)

	var mailTemplate string
	err = tx.QueryRow(context.Background(), "SELECT content FROM builder.mail_templates WHERE type = $1", mailData.GetMailType()).Scan(&mailTemplate)
	if err != nil {
		return uow, err
	}

	tmpl, err := template.New(string(mailData.GetMailType())).Parse(mailTemplate)
	if err != nil {
		return uow, err
	}
	var mailContent bytes.Buffer
	if err = tmpl.Execute(&mailContent, event.Data); err != nil {
		return uow, err
	}

	mail := db.Mail{
		MailType:   mailData.GetMailType(),
		Recipients: strings.Join(recipients, ","),
		Subject:    event.Subject,
		Content:    mailContent.String(),
		SentAt:     time.Now(),
	}
	_, err = tx.Exec(context.Background(), "INSERT INTO builder.mails(type, recipients, subject, content, sent_at) VALUES ($1,$2,$3,$4,$5)",
		mail.MailType, mail.Recipients, mail.Subject, mail.Content, mail.SentAt,
	)
	if err != nil {
		return uow, err
	}
	err = c.server.SendMail(recipients, mail.Subject, mail.Content)
	if err != nil {
		return uow, err
	}

	return uow, nil
}
