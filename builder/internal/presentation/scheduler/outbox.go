package scheduler

import (
	"context"
	"github.com/Builder-Lawyers/builder-backend/builder/internal/application/command"
	"github.com/Builder-Lawyers/builder-backend/builder/internal/infra/db"
	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	"log"
	"time"
)

type OutboxPoller struct {
	commands   command.Collection
	uowFactory dbs.UOWFactory
	limit      uint8
	interval   uint16
}

func NewOutboxPoller(commands command.Collection, uowFactory dbs.UOWFactory, limit uint8, interval uint16) *OutboxPoller {
	return &OutboxPoller{commands: commands, uowFactory: uowFactory, limit: limit, interval: interval}
}

func (o *OutboxPoller) Start() {
	ticker := time.NewTicker(time.Duration(o.interval) * time.Second)
	defer ticker.Stop()

	log.Println("Starting outbox poller...")
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
		log.Println("error in poller ", err)
	}
	query := "SELECT * FROM builder.outbox WHERE status = 0 ORDER BY created_at FOR NO KEY UPDATE LIMIT $1"
	events, err := tx.Query(context.Background(), query, o.limit)
	if err != nil {
		log.Println("error in poller ", err)
	}
	for events.Next() {
		var event db.Outbox
		if err := events.Scan(&event); err != nil {
			log.Println("error in poller ", err)
			continue
		}
		if err := o.handleEvent(event); err != nil {
			log.Println("error in poller ", err)
			continue
		}
		_, err := tx.Exec(context.Background(), "UPDATE builder.outbox SET status = 1 WHERE id = $1", event.ID)
		if err != nil {
			log.Println("error in poller ", err)
		}
		if err := tx.Commit(context.Background()); err != nil {
			log.Println("error in poller ", err)
		}
	}
	log.Println("Finished poller thread elaboration")
}

func (o *OutboxPoller) handleEvent(event db.Outbox) error {
	switch event.Event {
	case "SiteAwaitingProvision":

	}
}
