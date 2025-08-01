package events

import (
	"encoding/json"
	"time"
)

type SiteAwaitingProvision struct {
	ID             uint64
	SiteID         uint64
	TemplateName   string
	DomainVariants []string
	Fields         json.RawMessage
	CreatedAt      time.Time
}

func (s *SiteAwaitingProvision) GetType() string {
	return "SiteAwaitingProvision"
}
