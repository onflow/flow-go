package environment

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"
	otelTrace "go.opentelemetry.io/otel/trace"

	reusableRuntime "github.com/onflow/flow-go/fvm/runtime"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

// Environment implements the accounts business logic and exposes cadence
// runtime interface methods for the runtime.
type Environment interface {
	runtime.Interface

	AccountFreezer

	Chain() flow.Chain

	LimitAccountStorage() bool

	StartSpanFromRoot(name trace.SpanName) otelTrace.Span
	StartExtensiveTracingSpanFromRoot(name trace.SpanName) otelTrace.Span

	Logger() *zerolog.Logger

	BorrowCadenceRuntime() *reusableRuntime.ReusableCadenceRuntime
	ReturnCadenceRuntime(*reusableRuntime.ReusableCadenceRuntime)

	AccountsStorageCapacity(addresses []common.Address) (cadence.Value, error)

	Reset()
}
