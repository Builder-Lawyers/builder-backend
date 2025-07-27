package db

import (
	"encoding/json"
	"github.com/Builder-Lawyers/builder-backend/builder/internal/domain/consts"
	"github.com/Builder-Lawyers/builder-backend/builder/internal/domain/entity"
	"github.com/Builder-Lawyers/builder-backend/builder/internal/domain/events"
	"log"
)

func MapSiteModelToEntity(site Site, user User) entity.Site {
	return entity.Site{
		ID:         site.ID,
		TemplateID: site.TemplateID,
		Status:     consts.Status(site.Status),
		Creator:    MapUserModelToEntity(user),
	}
}

func MapUserModelToEntity(user User) entity.User {
	return entity.User{
		ID:           user.ID,
		Name:         user.FirstName,
		Surname:      user.SecondName,
		Email:        user.Email,
		RegisteredAt: user.CreatedAt,
	}
}

func MapOutboxModelToSiteAwaitingProvisionEvent(outbox Outbox) events.SiteAwaitingProvision {
	var payload struct {
		SiteID         uint64   `json:"site_id"`
		TemplateName   string   `json:"template_name"`
		DomainVariants []string `json:"domain_variants"`
	}

	if err := json.Unmarshal(outbox.Payload, &payload); err != nil {
		log.Println(err)
		return events.SiteAwaitingProvision{}
	}

	return events.SiteAwaitingProvision{
		ID:             outbox.ID,
		SiteID:         payload.SiteID,
		TemplateName:   payload.TemplateName,
		DomainVariants: payload.DomainVariants,
		CreatedAt:      outbox.CreatedAt,
	}
}

func RawMessageToMap(raw json.RawMessage) map[string]interface{} {
	var result map[string]interface{}
	if err := json.Unmarshal(raw, &result); err != nil {
		log.Println(err)
	}
	return result
}
