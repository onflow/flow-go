package data_provider

import (
	"context"

	"github.com/google/uuid"
)

type DataProvider interface {
	Run(ctx context.Context)
	ID() uuid.UUID
	Topic() string
	Close() error
}
