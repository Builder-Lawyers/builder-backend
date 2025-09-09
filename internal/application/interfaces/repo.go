package interfaces

import (
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db"
	"github.com/Builder-Lawyers/builder-backend/pkg/interfaces"
	"github.com/jackc/pgx/v5"
)

type ProvisionRepo interface {
	GetProvisionByID(tx pgx.Tx, siteID uint64) (db.Provision, error)
	InsertProvision(tx pgx.Tx, provision db.Provision) error
}

type EventRepo interface {
	InsertEvent(tx pgx.Tx, event interfaces.Event) error
}
