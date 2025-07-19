package db

import (
	"encoding/json"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/consts"
	"time"
)

type Outbox struct {
	ID        uint64              `db:"id"`
	Event     string              `db:"event"`
	Status    consts.OutboxStatus `db:"status"`
	Payload   json.RawMessage     `db:"payload"`
	CreatedAt time.Time           `db:"created_at"`
}
