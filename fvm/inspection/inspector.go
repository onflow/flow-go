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
	// - signers are the accounts that signed the procedure's transaction (its
	//   authorizers and payer), deduplicated. It is empty for procedures that are
	//   not transactions (e.g. scripts).
	Inspect(
		logger zerolog.Logger,
		storage snapshot.StorageSnapshot,
		executionSnapshot *snapshot.ExecutionSnapshot,
		events []flow.Event,
		signers []flow.Address,
	) (Result, error)

	// Name is the name of the inspector
	Name() string
}

// Result is the result of a procedure inspector
type Result interface {
	InspectionName() string
	AsLogEvent() (zerolog.Level, func(e *zerolog.Event))
}
