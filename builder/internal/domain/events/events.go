package events

import (
	"encoding/json"
	"time"
)

type SiteAwaitingProvision struct {
	ID        uint64
	SiteID    uint64
	Payload   json.RawMessage
	CreatedAt time.Time
}

func (s *SiteAwaitingProvision) GetType() string {
	return "SiteAwaitingProvision"
}
