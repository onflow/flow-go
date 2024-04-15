package util

import (
	"fmt"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
)

var _ storage.TransactionPreparer = migrationTransactionPreparer{}

func NewProgramsGetOrLoadProgramFunc(
	nestedTransactionPreparer state.NestedTransactionPreparer,
	accounts environment.Accounts,
) (GetOrLoadProgramFunc, error) {

	derivedChainData, err := derived.NewDerivedChainData(derived.DefaultDerivedDataCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create derived chain data: %w", err)
	}

	// The current block ID does not matter here, it is only for keeping a cross-block cache, which is not needed here.
	derivedTransactionData := derivedChainData.
		NewDerivedBlockDataForScript(flow.Identifier{}).
		NewSnapshotReadDerivedTransactionData()

	programs := environment.NewPrograms(
		tracing.NewTracerSpan(),
		NopMeter{},
		environment.NoopMetricsReporter{},
		migrationTransactionPreparer{
			NestedTransactionPreparer:  nestedTransactionPreparer,
			DerivedTransactionPreparer: derivedTransactionData,
		},
		accounts,
	)

	programErrors := map[common.Location]error{}

	return func(
		location runtime.Location,
		load func() (*interpreter.Program, error),
	) (
		program *interpreter.Program,
		err error,
	) {
		return programs.GetOrLoadProgram(
			location,
			func() (*interpreter.Program, error) {
				// If the program is already known to be invalid,
				// then return the error immediately,
				// without attempting to load the program again
				if err, ok := programErrors[location]; ok {
					return nil, err
				}

				// Otherwise, load the program.
				// If an error occurs, then record it for subsequent calls
				program, err := load()
				if err != nil {
					programErrors[location] = err
				}

				return program, err
			},
		)
	}, nil
}
