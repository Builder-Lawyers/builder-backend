package interfaces

import "github.com/jackc/pgx/v5"

type UoW interface {
	Commit() error
	Rollback() error
	Begin() (pgx.Tx, error)
}

type Event interface {
	GetType() string
}

type EventHandler interface {
	Handle(event Event) (any, error)
}
