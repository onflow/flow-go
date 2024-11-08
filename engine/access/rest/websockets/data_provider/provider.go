package data_provider

import (
	"github.com/google/uuid"
)

type DataProvider interface {
	Run()
	ID() uuid.UUID
	Topic() string
	Close()
}
