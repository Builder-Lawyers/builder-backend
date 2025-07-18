package entity

import (
	"github.com/google/uuid"
	"time"
)

type User struct {
	ID           uuid.UUID
	Name         string
	Surname      string
	Email        string
	RegisteredAt time.Time
}
