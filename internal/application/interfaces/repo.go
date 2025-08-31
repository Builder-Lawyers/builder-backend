package interfaces

import (
	"github.com/Builder-Lawyers/builder-backend/internal/db"
	"github.com/Builder-Lawyers/builder-backend/pkg/interfaces"
)

type ProvisionRepo interface {
	GetProvisionByID(tx pgx.Tx, siteID string) (db.Provision, error)
	InsertProvision(tx pgx.Tx, provision db.Provision) error
}

type EventRepo interface {
	InsertEvent(tx pgx.Tx, event interfaces.Event) error
}
