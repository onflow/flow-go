package inspection

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
)

// Inspector is run after each procedure on the procedure output and the starting state of a procedure
// It will then fill out the ProcedureOutput.Inspection results
type Inspector interface {
	// Inspect
	// - storage is the execution state before the procedure was executed.
	//    only the executionSnapshot.Reads, will be read
	// - executionSnapshot is the reads and writes of the procedure
	// - events are all of the events the procedure is emitting
	Inspect(
		logger zerolog.Logger,
		storage snapshot.StorageSnapshot,
		executionSnapshot *snapshot.ExecutionSnapshot,
		events []flow.Event,
	) (Result, error)
}

// Result is the result of a procedure inspector
type Result interface {
	AsLogEvent() (zerolog.Level, func(e *zerolog.Event))
}
