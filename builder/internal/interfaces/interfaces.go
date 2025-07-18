package interfaces

import "github.com/jackc/pgx/v5"

type UoW interface {
	Commit() error
	Rollback() error
	Begin() (pgx.Tx, error)
}
