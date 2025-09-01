package dto

import (
	"github.com/Builder-Lawyers/builder-backend/internal/domain/consts"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/mail"
)

type ProvisionSiteRequest struct {
	SiteID       uint64
	DomainType   consts.ProvisionType
	TemplateName string
	Domain       string
	Fields       map[string]interface{}
}

type CreateMailDTO struct {
	UserID  string
	Subject string
	Data    mail.MailData
}
