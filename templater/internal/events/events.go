package events

import (
	"encoding/json"
	"time"

	"github.com/Builder-Lawyers/builder-backend/templater/internal/consts"
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

func (s SiteAwaitingProvision) GetType() string {
	return "SiteAwaitingProvision"
}

type ProvisionCDN struct {
	SiteID         uint64
	OperationID    string
	CertificateARN string
	Domain         string
	CreatedAt      time.Time
}

func (s ProvisionCDN) GetType() string {
	return "ProvisionCDN"
}

type FinalizeProvision struct {
	SiteID         uint64
	DistributionID string
	Domain         string
	CreatedAt      time.Time
}

func (s FinalizeProvision) GetType() string {
	return "FinalizeProvision"
}
