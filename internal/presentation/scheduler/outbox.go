package scheduler

import (
	"context"
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
	return &OutboxPoller{handlers: handlers, uowFactory: uowFactory, cfg: cfg}
}

func (o *OutboxPoller) Start() {
	slog.Info("Starting outbox poller...")

	for {
		o.pollTable()
		// wait after poll finishes
		time.Sleep(time.Duration(o.cfg.interval) * time.Second)
	}
}

func (o *OutboxPoller) StartParallel() {
	ticker := time.NewTicker(time.Duration(o.cfg.interval) * time.Second)
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

	if err = uow.Commit(); err != nil {
		slog.Error("commit error", "err", err)
	}

	var wg sync.WaitGroup
	for _, event := range events {
		wg.Add(1)
		go func(ev db.Outbox) {
			defer wg.Done()
			if err := o.handleEvent(ev); err != nil {
				slog.Error("handler error", "event", ev.ID, "err", err)
			}
		}(event)
	}

	wg.Wait()
	slog.Info("Finished poller thread elaboration")
}

func (o *OutboxPoller) handleEvent(outbox db.Outbox) error {
	var (
		uow    interfaces.UoW
		tx     pgx.Tx
		err    error
		status = 1
	)

	switch outbox.Event {
	case events.SiteAwaitingProvision{}.GetType():
		event := db.MapOutboxModelToSiteAwaitingProvisionEvent(outbox)
		uow, err = o.handlers.ProvisionSite.Handle(event)
		if err != nil {
			status = 2
		}
		break
	case events.ProvisionCDN{}.GetType():
		event := db.MapOutboxModelToProvisionCDN(outbox)
		uow, err = o.handlers.ProvisionCDN.Handle(event)
		if err != nil {
			status = 2
		}
		break
	case events.FinalizeProvision{}.GetType():
		event := db.MapOutboxModelToFinalizeProvision(outbox)
		uow, err = o.handlers.FinalizeProvision.Handle(event)
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
		uow, err = o.handlers.SendMail.Handle(event)
		if err != nil {
			status = 2
		}
		break
	}

	if uow == nil {
		// open new transaction if there was no in event handler
		uow = o.uowFactory.GetUoW()
		tx, err = uow.Begin()
		if err != nil {
			return err
		}
	} else {
		tx = uow.GetTx()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err = tx.Exec(ctx, "UPDATE builder.outbox SET status = $1 WHERE id = $2", status, outbox.ID)
	if err != nil {
		_ = uow.Rollback()
		slog.Error("error in poller", "err", err)
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		slog.Error("error in poller", "err", err)
		return err
	}

	return nil
}
