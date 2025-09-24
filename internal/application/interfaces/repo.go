package interfaces

import (
	"context"

	"github.com/Builder-Lawyers/builder-backend/internal/infra/db"
	"github.com/Builder-Lawyers/builder-backend/pkg/interfaces"
)

type ProvisionRepo interface {
	GetProvisionByID(ctx context.Context, siteID uint64) (*db.Provision, error)
	InsertProvision(ctx context.Context, provision db.Provision) error
}

type EventRepo interface {
	InsertEvent(ctx context.Context, event interfaces.Event) error
}
