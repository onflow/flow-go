package inspection

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
)

type Inspector interface {
	Inspect(storage snapshot.StorageSnapshot, executionSnapshot *snapshot.ExecutionSnapshot, events []flow.Event) (Result, error)
}

type Result interface {
	AsLogEvent() (zerolog.Level, func(e *zerolog.Event))
}
