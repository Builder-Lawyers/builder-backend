package db

import (
	"encoding/json"
	"log"

	"github.com/Builder-Lawyers/builder-backend/internal/application/events"
	"github.com/Builder-Lawyers/builder-backend/internal/domain/consts"
	"github.com/Builder-Lawyers/builder-backend/internal/domain/entity"
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

func RawMessageToMap(raw json.RawMessage) map[string]interface{} {
	var result map[string]interface{}
	if err := json.Unmarshal(raw, &result); err != nil {
		log.Println(err)
	}
	return result
}

func MapOutboxModelToSiteAwaitingProvisionEvent(outbox Outbox) events.SiteAwaitingProvision {
	var payload struct {
		SiteID       uint64               `json:"siteID"`
		TemplateName string               `json:"templateName"`
		DomainType   consts.ProvisionType `json:"domainType"`
		Domain       string               `json:"domain"`
		Fields       json.RawMessage      `json:"fields"`
	}

	if err := json.Unmarshal(outbox.Payload, &payload); err != nil {
		log.Println(err)
		return events.SiteAwaitingProvision{}
	}

	return events.SiteAwaitingProvision{
		ID:           outbox.ID,
		SiteID:       payload.SiteID,
		TemplateName: payload.TemplateName,
		DomainType:   payload.DomainType,
		Domain:       payload.Domain,
		Fields:       payload.Fields,
		CreatedAt:    outbox.CreatedAt,
	}
}

func MapOutboxModelToProvisionCDN(outbox Outbox) events.ProvisionCDN {
	var payload struct {
		SiteID         uint64 `json:"siteID"`
		OperationID    string `json:"operationID"`
		CertificateARN string `json:"certificateARN"`
		Domain         string `json:"domain"`
	}

	if err := json.Unmarshal(outbox.Payload, &payload); err != nil {
		log.Println(err)
		return events.ProvisionCDN{}
	}

	return events.ProvisionCDN{
		SiteID:         payload.SiteID,
		OperationID:    payload.OperationID,
		CertificateARN: payload.CertificateARN,
		Domain:         payload.Domain,
		CreatedAt:      outbox.CreatedAt,
	}
}

func MapOutboxModelToFinalizeProvision(outbox Outbox) events.FinalizeProvision {
	var payload struct {
		SiteID         uint64 `json:"siteID"`
		DistributionID string `json:"distributionID"`
		Domain         string `json:"domain"`
		DomainType     string `json:"domainType"`
	}

	if err := json.Unmarshal(outbox.Payload, &payload); err != nil {
		log.Println(err)
		return events.FinalizeProvision{}
	}

	return events.FinalizeProvision{
		SiteID:         payload.SiteID,
		DistributionID: payload.DistributionID,
		Domain:         payload.Domain,
		DomainType:     consts.ProvisionType(payload.DomainType),
		CreatedAt:      outbox.CreatedAt,
	}
}

func MapToRawMessage(data map[string]interface{}) json.RawMessage {
	bytes, err := json.Marshal(data)
	if err != nil {
		log.Println(err)
		return nil
	}
	return json.RawMessage(bytes)
}
