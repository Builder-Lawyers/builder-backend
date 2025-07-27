package scheduler

import (
	"context"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/application"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/db"
	"log/slog"
	"time"
)

type OutboxPoller struct {
	commands   application.Commands
	uowFactory *dbs.UOWFactory
	limit      uint8
	interval   uint16
}

func NewOutboxPoller(commands application.Commands, uowFactory *dbs.UOWFactory, limit uint8, interval uint16) *OutboxPoller {
	return &OutboxPoller{commands: commands, uowFactory: uowFactory, limit: limit, interval: interval}
}

func (o *OutboxPoller) Start() {
	ticker := time.NewTicker(time.Duration(o.interval) * time.Second)
	defer ticker.Stop()

	slog.Info("Starting outbox poller...")
	for {
		select {
		case <-ticker.C:
			o.pollTable()
		}
	}
}

func (o *OutboxPoller) pollTable() {
	uow := o.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		slog.Error("error in poller", "err", err)
	}
	query := "SELECT * FROM builder.outbox WHERE status = 0 ORDER BY created_at FOR NO KEY UPDATE LIMIT $1"
	events, err := tx.Query(context.Background(), query, o.limit)
	if err != nil {
		slog.Error("error in poller", "err", err)

	}
	for events.Next() {
		var event db.Outbox
		if err = events.Scan(&event.ID, &event.Event, &event.Status, &event.Payload, &event.CreatedAt); err != nil {
			slog.Error("error in poller", "err", err)
			continue
		}
		if err = o.handleEvent(event); err != nil {
			slog.Error("error in poller", "err", err)
			continue
		}
		_, err = tx.Exec(context.Background(), "UPDATE builder.outbox SET status = 1 WHERE id = $1", event.ID)
		if err != nil {
			slog.Error("error in poller", "err", err)
		}
		if err = tx.Commit(context.Background()); err != nil {
			slog.Error("error in poller", "err", err)
		}
	}
	if err = tx.Commit(context.Background()); err != nil {
		slog.Error("error in poller", "err", err)
	}
	slog.Info("Finished poller thread elaboration")
}

func (o *OutboxPoller) handleEvent(outbox db.Outbox) error {
	switch outbox.Event {
	case "SiteAwaitingProvision":
		event := db.MapOutboxModelToSiteAwaitingProvisionEvent(outbox)
		err := o.commands.ProvisionSite.Handle(event)
		if err != nil {
			return err
		}
	}

	return nil
}
