package db

import (
	"encoding/json"
	"log"

	"github.com/Builder-Lawyers/builder-backend/templater/internal/consts"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/events"
)

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
	}

	if err := json.Unmarshal(outbox.Payload, &payload); err != nil {
		log.Println(err)
		return events.FinalizeProvision{}
	}

	return events.FinalizeProvision{
		SiteID:         payload.SiteID,
		DistributionID: payload.DistributionID,
		Domain:         payload.Domain,
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
