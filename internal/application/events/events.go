package events

import (
	"encoding/json"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application/consts"
)

type SiteAwaitingProvision struct {
	ID           uint64
	SiteID       uint64
	TemplateName string
	DomainType   consts.ProvisionType
	Domain       string
	Fields       json.RawMessage
	CreatedAt    time.Time
}

func (e SiteAwaitingProvision) GetType() string {
	return "SiteAwaitingProvision"
}

type ProvisionCDN struct {
	SiteID         uint64
	OperationID    string
	CertificateARN string
	Domain         string
	CreatedAt      time.Time
}

func (e ProvisionCDN) GetType() string {
	return "ProvisionCDN"
}

type FinalizeProvision struct {
	SiteID         uint64
	DistributionID string
	DomainType     consts.ProvisionType
	Domain         string
	CreatedAt      time.Time
}

func (e FinalizeProvision) GetType() string {
	return "FinalizeProvision"
}

type SendMail struct {
	UserID  string
	Subject string
	Data    interface{}
}

func (e SendMail) GetType() string {
	return "SendMail"
}
