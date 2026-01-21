package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application/consts"
	"github.com/Builder-Lawyers/builder-backend/internal/application/interfaces"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db"
	shared "github.com/Builder-Lawyers/builder-backend/pkg/interfaces"
	"github.com/jackc/pgx/v5"
)

type ProvisionRepo struct {
	tx pgx.Tx
}

var _ interfaces.ProvisionRepo = (*ProvisionRepo)(nil)

func NewProvisionRepo(tx pgx.Tx) *ProvisionRepo {
	return &ProvisionRepo{tx: tx}
}

func (p *ProvisionRepo) GetProvisionByID(ctx context.Context, siteID uint64) (*db.Provision, error) {
	var provision db.Provision
	query := "SELECT site_id, type, status, domain, cert_arn, cloudfront_id, structure_path, created_at, updated_at FROM builder.provisions WHERE site_id = $1"
	err := p.tx.QueryRow(ctx, query, siteID).Scan(&provision.SiteID, &provision.Type, &provision.Status,
		&provision.Domain, &provision.CertificateARN, &provision.CloudfrontID, &provision.StructurePath, &provision.CreatedAt, &provision.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &provision, nil
}

func (p *ProvisionRepo) InsertProvision(ctx context.Context, provision db.Provision) error {
	_, err := p.tx.Exec(ctx, `INSERT INTO builder.provisions(site_id, type, status, domain, cert_arn, cloudfront_id, structure_path, created_at, updated_at) 
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, provision.SiteID, provision.Type, provision.Status, provision.Domain, provision.CertificateARN,
		provision.CloudfrontID, provision.StructurePath, provision.CreatedAt, provision.UpdatedAt)
	if err != nil {
		return err
	}

	return nil
}

type EventRepo struct {
	tx pgx.Tx
}

var _ interfaces.EventRepo = (*EventRepo)(nil)

func NewEventRepo(tx pgx.Tx) *EventRepo {
	return &EventRepo{tx: tx}
}

func (e *EventRepo) InsertEvent(ctx context.Context, event shared.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("err marshalling event payload, %v", err)
	}
	outbox := db.Outbox{
		Event:     event.GetType(),
		Status:    int(consts.NotProcessed),
		Payload:   json.RawMessage(payload),
		CreatedAt: time.Now(),
	}
	_, err = e.tx.Exec(ctx, "INSERT INTO builder.outbox (event, status, payload, created_at) VALUES ($1,$2,$3,$4)",
		outbox.Event, outbox.Status, outbox.Payload, outbox.CreatedAt)
	if err != nil {
		return fmt.Errorf("err inserting a new event, %v", err)
	}

	return nil
}
