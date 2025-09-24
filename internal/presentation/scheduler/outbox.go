package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application"
	"github.com/Builder-Lawyers/builder-backend/internal/application/events"
	"github.com/Builder-Lawyers/builder-backend/internal/infra/db"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/Builder-Lawyers/builder-backend/pkg/env"
	"github.com/Builder-Lawyers/builder-backend/pkg/interfaces"
	"github.com/jackc/pgx/v5"
)

type OutboxPoller struct {
	handlers   *application.Handlers
	uowFactory *dbs.UOWFactory
	cfg        *OutboxConfig
	stop       chan struct{}
}

type OutboxConfig struct {
	limit    uint8
	interval uint16
}

func NewOutboxConfig() *OutboxConfig {
	var limit int
	var interval int

	limitString := env.GetEnv("SCHEDULER_LIMIT", "5")
	limit, err := strconv.Atoi(limitString)
	if err != nil {
		limit = 5
	}

	intervalString := env.GetEnv("SCHEDULER_INTERVAL", "5")
	interval, err = strconv.Atoi(intervalString)
	if err != nil {
		interval = 5
	}
	return &OutboxConfig{
		limit:    uint8(limit),
		interval: uint16(interval),
	}
}

func NewOutboxPoller(handlers *application.Handlers, uowFactory *dbs.UOWFactory, cfg *OutboxConfig) *OutboxPoller {
	return &OutboxPoller{handlers: handlers, uowFactory: uowFactory, cfg: cfg, stop: make(chan struct{})}
}

func (o *OutboxPoller) Start() {
	slog.Info("Starting outbox poller...")
	t := time.NewTimer(time.Duration(o.cfg.interval) * time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	for {
		select {
		case <-t.C:
			o.pollTable(ctx)
			t = time.NewTimer(time.Duration(o.cfg.interval) * time.Second)
		case <-o.stop:
			slog.Info("Cancelling current execution")
			cancel()
		}
		// wait after poll finishes
	}
}

func (o *OutboxPoller) StartParallel() {
	ticker := time.NewTicker(time.Duration(o.cfg.interval) * time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer ticker.Stop()

	slog.Info("Starting outbox poller...")
	for {
		select {
		case <-ticker.C:
			go o.pollTable(ctx)
		case <-o.stop:
			slog.Info("Cancelling current execution")
			cancel()
		}
	}
}

func (o *OutboxPoller) pollTable(ctx context.Context) {
	uow := o.uowFactory.GetUoW()
	tx, err := uow.Begin()
	if err != nil {
		slog.Error("error in poller", "err", err)
		return
	}

	query := "SELECT * FROM builder.outbox WHERE status = 0 ORDER BY created_at FOR NO KEY UPDATE LIMIT $1"
	rows, err := tx.Query(ctx, query, o.cfg.limit)
	if err != nil {
		slog.Error("error in poller", "err", err)
		return
	}

	defer rows.Close()
	var events []db.Outbox
	for rows.Next() {
		var event db.Outbox
		if err = rows.Scan(&event.ID, &event.Event, &event.Status, &event.Payload, &event.CreatedAt); err != nil {
			slog.Error("error in poller", "err", err)
			continue
		}
		events = append(events, event)
	}

	if err = rows.Err(); err != nil {
		slog.Error("error reading result sets, ", "err", err)
	}

	if err = uow.Commit(); err != nil {
		slog.Error("commit error", "err", err)
	}

	var wg sync.WaitGroup
	for _, event := range events {
		wg.Add(1)
		go func(ev db.Outbox) {
			defer wg.Done()
			if err := o.handleEvent(ctx, ev); err != nil {
				slog.Error("handler error", "event", ev.ID, "err", err)
			}
		}(event)
	}

	wg.Wait()
	slog.Info("Finished poller thread elaboration")
}

func (o *OutboxPoller) handleEvent(ctx context.Context, outbox db.Outbox) error {
	var (
		uow    interfaces.UoW
		tx     pgx.Tx
		err    error
		status = 1
	)

	slog.Info("Handling event", "event", outbox.Event, "id", outbox.ID)

	switch outbox.Event {
	case events.SiteAwaitingProvision{}.GetType():
		event := db.MapOutboxModelToSiteAwaitingProvisionEvent(outbox)
		uow, err = o.handlers.ProvisionSite.Handle(ctx, event)
		if err != nil {
			status = 2
		}
		break
	case events.ProvisionCDN{}.GetType():
		event := db.MapOutboxModelToProvisionCDN(outbox)
		uow, err = o.handlers.ProvisionCDN.Handle(ctx, event)
		if err != nil {
			status = 2
		}
		break
	case events.FinalizeProvision{}.GetType():
		event := db.MapOutboxModelToFinalizeProvision(outbox)
		uow, err = o.handlers.FinalizeProvision.Handle(ctx, event)
		if err != nil {
			if strings.Contains(err.Error(), "timed out waiting for distribution to deploy") {
				slog.Warn("Distribution still deploying, will retry later")
				return nil
			}
			status = 2
		}
		break
	case events.SendMail{}.GetType():
		event := db.MapOutboxModelToSendMail(outbox)
		uow, err = o.handlers.SendMail.Handle(ctx, event)
		if err != nil {
			status = 2
		}
		break
	case events.DeactivateSite{}.GetType():
		event := db.MapOutboxModelToDeactivateSite(outbox)
		uow, err = o.handlers.DeactivateSite.Handle(ctx, event)
		if err != nil {
			status = 2
		}
		break
	}

	if err != nil {
		slog.Error("error in handler", "event", outbox.Event, "id", outbox.ID, "err", err)
	}

	if uow == nil {
		var errTx error
		// open new transaction if there was no in event handler
		uow = o.uowFactory.GetUoW()
		tx, errTx = uow.Begin()
		if errTx != nil {
			return errors.Join(err, errTx)
		}
	} else {
		tx = uow.GetTx()
	}

	_, err = tx.Exec(ctx, "UPDATE builder.outbox SET status = $1 WHERE id = $2", status, outbox.ID)
	if err != nil {
		errRollback := uow.Rollback()
		slog.Error("error in poller", "err", err)
		return errors.Join(err, errRollback)
	}

	if err = uow.Commit(); err != nil {
		slog.Error("error in poller", "err", err)
		return err
	}

	return nil
}

func (o *OutboxPoller) Stop() {
	slog.Info("Stopping poller")
	o.stop <- struct{}{}
}
