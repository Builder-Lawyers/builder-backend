package db

import "time"

type Outbox struct {
	ID        uint64              `db:"id"`
	Event     string              `db:"event"`
	Status    consts.OutboxStatus `db:"status"`
	CreatedAt time.Time           `db:"created_at"`
}
