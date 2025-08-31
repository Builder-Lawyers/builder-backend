package db

import (
	"encoding/json"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/domain/consts"
	"github.com/google/uuid"
)

type Site struct {
	ID         uint64          `db:"id"`
	TemplateID uint8           `db:"template_id"`
	CreatorID  uuid.UUID       `db:"creator_id"`
	Status     string          `db:"status"`
	Fields     json.RawMessage `db:"fields"`
	CreatedAt  time.Time       `db:"created_at"`
	UpdatedAt  time.Time       `db:"updated_at,omitempty"`
}

type User struct {
	ID         uuid.UUID `db:"id"`
	FirstName  string    `db:"first_name"`
	SecondName string    `db:"second_name"`
	Email      string    `db:"email"`
	CreatedAt  time.Time `db:"created_at,omitempty"`
}

type Template struct {
	ID   uint8  `db:"id"`
	Name string `db:"name"`
}

type Outbox struct {
	ID        uint64          `db:"id"`
	Event     string          `db:"event"`
	Status    int             `db:"status"`
	Payload   json.RawMessage `db:"payload"`
	CreatedAt time.Time       `db:"created_at"`
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
