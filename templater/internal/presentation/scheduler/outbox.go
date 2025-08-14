package scheduler

import (
	"context"
	"log/slog"
	"time"

	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/application"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/db"
	"github.com/Builder-Lawyers/builder-backend/templater/internal/events"
	"github.com/jackc/pgx/v5"
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
			go o.pollTable()
		}
	}
}

func (o *OutboxPoller) pollTable() {
	uow := o.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		slog.Error("error in poller", "err", err)
		return
	}

	timeout := 2 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	query := "SELECT * FROM builder.outbox WHERE status = 0 ORDER BY created_at FOR NO KEY UPDATE LIMIT $1"
	events, err := tx.Query(ctx, query, o.limit)
	if err != nil {
		slog.Error("error in poller", "err", err)
		return
	}
	defer events.Close()
	//defer tx.Conn().Close(context.Background())
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
	}

	ctxCommit, cancelCommit := context.WithTimeout(context.Background(), timeout)
	defer cancelCommit()
	if err = tx.Commit(ctxCommit); err != nil {
		slog.Error("error in poller", "err", err)
	}
	slog.Info("Finished poller thread elaboration")
}

func (o *OutboxPoller) handleEvent(outbox db.Outbox) error {
	var (
		tx     pgx.Tx
		err    error
		status = 1
	)

	switch outbox.Event {
	case events.SiteAwaitingProvision{}.GetType():
		event := db.MapOutboxModelToSiteAwaitingProvisionEvent(outbox)
		tx, err = o.commands.ProvisionSite.Handle(event)
		if err != nil {
			status = 2
		}
		break
	case events.ProvisionCDN{}.GetType():
		event := db.MapOutboxModelToProvisionCDN(outbox)
		tx, err = o.commands.ProvisionCDN.Handle(event)
		if err != nil {
			status = 2
		}
		break
	case events.FinalizeProvision{}.GetType():
		event := db.MapOutboxModelToFinalizeProvision(outbox)
		tx, err = o.commands.FinalizeProvision.Handle(event)
		if err != nil {
			status = 2
		}
		break
	}

	if tx == nil {
		// open new transaction if there was no in event handler
		uow := o.uowFactory.GetUoW()
		tx, err = uow.Begin()
		if err != nil {
			return err
		}
	}

	timeout := 3 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, err = tx.Exec(ctx, "UPDATE builder.outbox SET status = $1 WHERE id = $2", status, outbox.ID)
	if err != nil {
		slog.Error("error in poller", "err", err)
	}

	if err = tx.Commit(ctx); err != nil {
		slog.Error("error in poller", "err", err)
	}

	return nil
}
