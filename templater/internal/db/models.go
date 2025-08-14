package db

import (
	"encoding/json"
	"time"

	"github.com/Builder-Lawyers/builder-backend/templater/internal/consts"
)

type Outbox struct {
	ID        uint64              `db:"id"`
	Event     string              `db:"event"`
	Status    consts.OutboxStatus `db:"status"`
	Payload   json.RawMessage     `db:"payload"`
	CreatedAt time.Time           `db:"created_at"`
}

type Provision struct {
	SiteID         uint64                 `db:"site_id"`
	Type           consts.ProvisionType   `db:"type"`
	Status         consts.ProvisionStatus `db:"status"`
	Domain         string                 `db:"domain"`
	CertificateARN string                 `db:"cert_arn"`
	CloudfrontID   string                 `db:"cloudfront_id"`
	CreatedAt      time.Time              `db:"created_at"`
	UpdatedAt      time.Time              `db:"updated_at"`
}
