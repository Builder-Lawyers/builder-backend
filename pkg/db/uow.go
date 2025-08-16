package db

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Builder-Lawyers/builder-backend/pkg/interfaces"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UOW struct {
	Pool *pgxpool.Pool
	Conn *pgxpool.Conn
	Tx   pgx.Tx
}

var _ interfaces.UoW = (*UOW)(nil)

func (u *UOW) Begin() (pgx.Tx, error) {
	conn, err := u.Pool.Acquire(context.Background())
	if err != nil {
		return nil, fmt.Errorf("can't acquire conn, %w", err)
	}
	slog.Info("acquired conn", "pid", conn.Conn().PgConn().PID())

	tx, err := conn.BeginTx(context.Background(), pgx.TxOptions{})
	if err != nil {
		conn.Release()
		return nil, fmt.Errorf("can't begin tx, %w", err)
	}
	u.Tx = tx
	u.Conn = conn
	return u.Tx, nil
}

func (u *UOW) Commit() error {
	if u.Tx == nil {
		return fmt.Errorf("transaction is not started yet")
	}
	defer u.Conn.Release()
	return u.Tx.Commit(context.Background())
}

func (u *UOW) Rollback() error {
	if u.Tx == nil {
		return fmt.Errorf("transaction is not started yet")
	}
	defer u.Conn.Release()
	return u.Tx.Rollback(context.Background())
}

func (u *UOW) GetTx() pgx.Tx {
	return u.Tx
}

type UOWFactory struct {
	Pool *pgxpool.Pool
}

func (u *UOWFactory) GetUoW() interfaces.UoW {
	return &UOW{
		Pool: u.Pool,
	}
}

func NewUoWFactory(pool *pgxpool.Pool) *UOWFactory {
	return &UOWFactory{
		Pool: pool,
	}
}
