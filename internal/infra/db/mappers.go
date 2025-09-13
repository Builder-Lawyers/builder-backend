package db

import (
	"encoding/json"
	"log/slog"

	"github.com/Builder-Lawyers/builder-backend/internal/application/events"
)

func RawMessageToMap(raw json.RawMessage) map[string]interface{} {
	var result map[string]interface{}
	if err := json.Unmarshal(raw, &result); err != nil {
		slog.Error("error unmarshaling event", "err", err)
	}
	return result
}

func MapOutboxModelToSiteAwaitingProvisionEvent(outbox Outbox) events.SiteAwaitingProvision {
	var siteAwaitingProvision events.SiteAwaitingProvision
	if err := json.Unmarshal(outbox.Payload, &siteAwaitingProvision); err != nil {
		slog.Error("error unmarshaling event", "err", err)
		return events.SiteAwaitingProvision{}
	}

	return siteAwaitingProvision
}

func MapOutboxModelToProvisionCDN(outbox Outbox) events.ProvisionCDN {
	var provisionCDN events.ProvisionCDN
	if err := json.Unmarshal(outbox.Payload, &provisionCDN); err != nil {
		slog.Error("error unmarshaling event", "err", err)
		return events.ProvisionCDN{}
	}
	provisionCDN.CreatedAt = outbox.CreatedAt

	return provisionCDN
}

func MapOutboxModelToFinalizeProvision(outbox Outbox) events.FinalizeProvision {
	var finalizeProvision events.FinalizeProvision
	if err := json.Unmarshal(outbox.Payload, &finalizeProvision); err != nil {
		slog.Error("error unmarshaling event", "err", err)
		return events.FinalizeProvision{}
	}
	finalizeProvision.CreatedAt = outbox.CreatedAt

	return finalizeProvision
}

func MapOutboxModelToSendMail(outbox Outbox) events.SendMail {
	var payload struct {
		UserID  string      `json:"userID"`
		Subject string      `json:"subject"`
		Data    interface{} `json:"data"`
	}

	if err := json.Unmarshal(outbox.Payload, &payload); err != nil {
		slog.Error("error unmarshaling event", "err", err)
		return events.SendMail{}
	}

	return events.SendMail{
		UserID:  payload.UserID,
		Subject: payload.Subject,
		Data:    payload.Data,
	}
}

func MapOutboxModelToDeactivateSite(outbox Outbox) events.DeactivateSite {
	var deactivateSite events.DeactivateSite
	if err := json.Unmarshal(outbox.Payload, &deactivateSite); err != nil {
		slog.Error("error unmarshaling event", "err", err)
		return events.DeactivateSite{}
	}

	return deactivateSite
}

func MapToRawMessage(data map[string]interface{}) json.RawMessage {
	bytes, err := json.Marshal(data)
	if err != nil {
		slog.Error("error unmarshaling event", "err", err)
		return nil
	}
	return json.RawMessage(bytes)
}
