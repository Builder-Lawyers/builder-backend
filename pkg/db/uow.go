package db

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
)

type UOW struct {
	Conn Connection
	Tx   pgx.Tx
}

func (u UOW) Begin() (pgx.Tx, error) {
	tx, err := u.Conn.BeginTx(context.Background(), pgx.TxOptions{DeferrableMode: pgx.Deferrable})
	if err != nil {
		return nil, fmt.Errorf("can't begin tx, %v", err)
	}
	u.Tx = tx
	return u.Tx, nil
}

func (u UOW) Commit() error {
	if u.Tx == nil {
		return fmt.Errorf("transaction is not started yet")
	}
	return u.Tx.Commit(context.Background())
}

func (u UOW) Rollback() error {
	if u.Tx == nil {
		return fmt.Errorf("transaction is not started yet")
	}
	return u.Tx.Rollback(context.Background())
}

type UOWFactory struct {
	Conn Connection
}

func (u *UOWFactory) GetUoW() *UOW {
	return &UOW{
		Conn: u.Conn,
	}
}

func NewUoWFactory(conn Connection) *UOWFactory {
	return &UOWFactory{
		Conn: conn,
	}
}
