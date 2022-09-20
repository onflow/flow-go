package environment

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/fvm/programs"
	reusableRuntime "github.com/onflow/flow-go/fvm/runtime"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

// Environment implements the accounts business logic and exposes cadence
// runtime interface methods for the runtime.
type Environment interface {
	runtime.Interface

	// Tracer
	StartSpanFromRoot(name trace.SpanName) otelTrace.Span

	Meter

	// Runtime
	BorrowCadenceRuntime() *reusableRuntime.ReusableCadenceRuntime
	ReturnCadenceRuntime(*reusableRuntime.ReusableCadenceRuntime)

	TransactionInfo

	// ProgramLogger
	Logs() []string

	// EventEmitter
	Events() []flow.Event
	ServiceEvents() []flow.Event

	// SystemContracts
	AccountsStorageCapacity(addresses []common.Address) (cadence.Value, error)
	DeductTransactionFees(
		payer flow.Address,
		inclusionEffort uint64,
		executionEffort uint64,
	) (
		cadence.Value,
		error,
	)

	AccountFreezer

	// FlushPendingUpdates flushes pending updates from the stateful environment
	// modules (i.e., ContractUpdater) to the state transaction, and return
	// corresponding modified sets invalidator.
	FlushPendingUpdates() (
		programs.ModifiedSetsInvalidator,
		error,
	)

	// Reset resets all stateful environment modules (e.g., ContractUpdater,
	// EventEmitter, AccountFreezer) to initial state.
	Reset()
}
