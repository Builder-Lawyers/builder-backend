package dto

import "github.com/Builder-Lawyers/builder-backend/internal/domain/consts"

type ProvisionSiteRequest struct {
	SiteID       uint64
	DomainType   consts.ProvisionType
	TemplateName string
	Domain       string
	Fields       map[string]interface{}
}
