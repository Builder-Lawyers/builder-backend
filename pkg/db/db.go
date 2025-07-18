package db

import (
	"context"
	"github.com/jackc/pgx/v5"
	"log"
)

type Connection struct {
	*pgx.Conn
}

func New(config Config) Connection {
	conn, err := pgx.Connect(context.Background(), config.GetDSN())
	if err != nil {
		log.Fatalln("error creating conn ", err)
	}
	return Connection{
		conn,
	}
}
