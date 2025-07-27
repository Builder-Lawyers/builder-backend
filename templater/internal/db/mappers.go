package db

import (
	"encoding/json"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/events"
	"log"
)

func MapOutboxModelToSiteAwaitingProvisionEvent(outbox Outbox) events.SiteAwaitingProvision {
	var payload struct {
		SiteID         uint64          `json:"siteID"`
		TemplateName   string          `json:"templateName"`
		DomainVariants []string        `json:"domainVariants"`
		Fields         json.RawMessage `json:"fields"`
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
		Fields:         payload.Fields,
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

func MapToRawMessage(data map[string]interface{}) json.RawMessage {
	bytes, err := json.Marshal(data)
	if err != nil {
		log.Println(err)
		return nil
	}
	return json.RawMessage(bytes)
}
