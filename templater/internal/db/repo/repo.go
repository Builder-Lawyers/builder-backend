package repo

import (
	"context"
	"encoding/json"
	"time"

	shared "github.com/Builder-Lawyers/builder-backend/pkg/interfaces"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/application/interfaces"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/consts"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/db"
	"github.com/jackc/pgx/v5"
)

type ProvisionRepo struct {
}

var _ interfaces.ProvisionRepo = (*ProvisionRepo)(nil)

func NewProvisionRepo() *ProvisionRepo {
	return &ProvisionRepo{}
}

func (p ProvisionRepo) GetProvisionByID(tx pgx.Tx, siteID string) (db.Provision, error) {
	var provision db.Provision
	err := tx.QueryRow(context.Background(), "SELECT site_id FROM builder.provisions WHERE site_id = $1", siteID).Scan(&provision.SiteID,
		&provision.Type, &provision.Domain, &provision.CertificateARN, &provision.CloudfrontID, &provision.CreatedAt, &provision.UpdatedAt)
	if err != nil {
		return db.Provision{}, err
	}

	return provision, nil
}

func (p ProvisionRepo) InsertProvision(tx pgx.Tx, provision db.Provision) error {
	_, err := tx.Exec(context.Background(), "INSERT INTO builder.provisions(site_id, type, status, domain, cert_arn, cloudfront_id, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)", provision.SiteID, provision.Type, provision.Status, provision.Domain, provision.CertificateARN,
		provision.CloudfrontID, provision.CreatedAt, provision.UpdatedAt)
	if err != nil {
		return err
	}

	return nil
}

type EventRepo struct {
}

var _ interfaces.EventRepo = (*EventRepo)(nil)

func NewEventRepo() *EventRepo {
	return &EventRepo{}
}

func (e EventRepo) InsertEvent(tx pgx.Tx, event shared.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	outbox := db.Outbox{
		Event:     event.GetType(),
		Status:    consts.NotProcessed,
		Payload:   json.RawMessage(payload),
		CreatedAt: time.Now(),
	}
	_, err = tx.Exec(context.Background(), "INSERT INTO builder.outbox (event, status, payload, created_at) VALUES ($1,$2,$3,$4)",
		outbox.Event, outbox.Status, outbox.Payload, outbox.CreatedAt)
	if err != nil {
		return err
	}

	return nil
}
