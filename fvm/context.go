package fvm

import (
	"math"

	"github.com/rs/zerolog"
	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/fvm/environment"
	reusableRuntime "github.com/onflow/flow-go/fvm/runtime"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

const (
	AccountKeyWeightThreshold = 1000

	DefaultComputationLimit   = 100_000 // 100K
	DefaultMemoryLimit        = math.MaxUint64
	DefaultMaxInteractionSize = 20_000_000 // ~20MB
)

// A Context defines a set of execution parameters used by the virtual machine.
type Context struct {
	// DisableMemoryAndInteractionLimits will override memory and interaction
	// limits and set them to MaxUint64, effectively disabling these limits.
	DisableMemoryAndInteractionLimits bool
	EVMEnabled                        bool
	ComputationLimit                  uint64
	MemoryLimit                       uint64
	MaxStateKeySize                   uint64
	MaxStateValueSize                 uint64
	MaxStateInteractionSize           uint64

	TransactionExecutorParams

	DerivedBlockData *derived.DerivedBlockData

	tracing.TracerSpan

	environment.EnvironmentParams
}

// NewContext initializes a new execution context with the provided options.
func NewContext(opts ...Option) Context {
	return newContext(defaultContext(), opts...)
}

// NewContextFromParent spawns a child execution context with the provided options.
func NewContextFromParent(parent Context, opts ...Option) Context {
	return newContext(parent, opts...)
}

func newContext(ctx Context, opts ...Option) Context {
	for _, applyOption := range opts {
		ctx = applyOption(ctx)
	}

	return ctx
}

func defaultContext() Context {
	return Context{
		DisableMemoryAndInteractionLimits: false,
		ComputationLimit:                  DefaultComputationLimit,
		MemoryLimit:                       DefaultMemoryLimit,
		MaxStateKeySize:                   state.DefaultMaxKeySize,
		MaxStateValueSize:                 state.DefaultMaxValueSize,
		MaxStateInteractionSize:           DefaultMaxInteractionSize,
		TransactionExecutorParams:         DefaultTransactionExecutorParams(),
		EnvironmentParams:                 environment.DefaultEnvironmentParams(),
	}
}

// An Option sets a configuration parameter for a virtual machine context.
type Option func(ctx Context) Context

// WithChain sets the chain parameters for a virtual machine context.
func WithChain(chain flow.Chain) Option {
	return func(ctx Context) Context {
		ctx.Chain = chain
		return ctx
	}
}

// WithGasLimit sets the computation limit for a virtual machine context.
// @depricated, please use WithComputationLimit instead.
func WithGasLimit(limit uint64) Option {
	return func(ctx Context) Context {
		ctx.ComputationLimit = limit
		return ctx
	}
}

// WithMemoryAndInteractionLimitsDisabled will override memory and interaction
// limits and set them to MaxUint64, effectively disabling these limits.
func WithMemoryAndInteractionLimitsDisabled() Option {
	return func(ctx Context) Context {
		ctx.DisableMemoryAndInteractionLimits = true
		return ctx
	}
}

// WithComputationLimit sets the computation limit for a virtual machine context.
func WithComputationLimit(limit uint64) Option {
	return func(ctx Context) Context {
		ctx.ComputationLimit = limit
		return ctx
	}
}

// WithMemoryLimit sets the memory limit for a virtual machine context.
func WithMemoryLimit(limit uint64) Option {
	return func(ctx Context) Context {
		ctx.MemoryLimit = limit
		return ctx
	}
}

// WithLogger sets the context logger
func WithLogger(logger zerolog.Logger) Option {
	return func(ctx Context) Context {
		ctx.Logger = logger
		return ctx
	}
}

// WithMaxStateKeySize sets the byte size limit for ledger keys
func WithMaxStateKeySize(limit uint64) Option {
	return func(ctx Context) Context {
		ctx.MaxStateKeySize = limit
		return ctx
	}
}

// WithMaxStateValueSize sets the byte size limit for ledger values
func WithMaxStateValueSize(limit uint64) Option {
	return func(ctx Context) Context {
		ctx.MaxStateValueSize = limit
		return ctx
	}
}

// WithMaxStateInteractionSize sets the byte size limit for total interaction with ledger.
// this prevents attacks such as reading all large registers
func WithMaxStateInteractionSize(limit uint64) Option {
	return func(ctx Context) Context {
		ctx.MaxStateInteractionSize = limit
		return ctx
	}
}

// WithEventCollectionSizeLimit sets the event collection byte size limit for a virtual machine context.
func WithEventCollectionSizeLimit(limit uint64) Option {
	return func(ctx Context) Context {
		ctx.EventCollectionByteSizeLimit = limit
		return ctx
	}
}

// WithEntropyProvider sets the entropy provider of a virtual machine context.
//
// The VM uses the input to provide entropy to the Cadence runtime randomness functions.
func WithEntropyProvider(source environment.EntropyProvider) Option {
	return func(ctx Context) Context {
		ctx.EntropyProvider = source
		return ctx
	}
}

// WithBlockHeader sets the block header for a virtual machine context.
//
// The VM uses the header to provide current block information to the Cadence runtime.
func WithBlockHeader(header *flow.Header) Option {
	return func(ctx Context) Context {
		ctx.BlockHeader = header
		return ctx
	}
}

// WithServiceEventCollectionEnabled enables service event collection
func WithServiceEventCollectionEnabled() Option {
	return func(ctx Context) Context {
		ctx.ServiceEventCollectionEnabled = true
		return ctx
	}
}

// WithBlocks sets the block storage provider for a virtual machine context.
//
// The VM uses the block storage provider to provide historical block information to
// the Cadence runtime.
func WithBlocks(blocks environment.Blocks) Option {
	return func(ctx Context) Context {
		ctx.Blocks = blocks
		return ctx
	}
}

// WithMetricsReporter sets the metrics collector for a virtual machine context.
//
// A metrics collector is used to gather metrics reported by the Cadence runtime.
func WithMetricsReporter(mr environment.MetricsReporter) Option {
	return func(ctx Context) Context {
		if mr != nil {
			ctx.MetricsReporter = mr
		}
		return ctx
	}
}

// WithTracer sets the tracer for a virtual machine context.
func WithTracer(tr module.Tracer) Option {
	return func(ctx Context) Context {
		ctx.Tracer = tr
		return ctx
	}
}

// WithSpan sets the trace span for a virtual machine context.
func WithSpan(span otelTrace.Span) Option {
	return func(ctx Context) Context {
		ctx.Span = span
		return ctx
	}
}

// WithExtensiveTracing sets the extensive tracing
func WithExtensiveTracing() Option {
	return func(ctx Context) Context {
		ctx.ExtensiveTracing = true
		return ctx
	}
}

// WithServiceAccount enables or disables calls to the Flow service account.
func WithServiceAccount(enabled bool) Option {
	return func(ctx Context) Context {
		ctx.ServiceAccountEnabled = enabled
		return ctx
	}
}

// WithRestrictContractRemoval enables or disables restricted contract removal for a
// virtual machine context. Warning! this would be overridden with the flag stored on chain.
// this is just a fallback value
func WithContractRemovalRestricted(enabled bool) Option {
	return func(ctx Context) Context {
		ctx.RestrictContractRemoval = enabled
		return ctx
	}
}

// WithRestrictedContractDeployment enables or disables restricted contract deployment for a
// virtual machine context. Warning! this would be overridden with the flag stored on chain.
// this is just a fallback value
func WithContractDeploymentRestricted(enabled bool) Option {
	return func(ctx Context) Context {
		ctx.RestrictContractDeployment = enabled
		return ctx
	}
}

// WithCadenceLogging enables or disables Cadence logging for a
// virtual machine context.
func WithCadenceLogging(enabled bool) Option {
	return func(ctx Context) Context {
		ctx.CadenceLoggingEnabled = enabled
		return ctx
	}
}

// WithAccountStorageLimit enables or disables checking if account storage used is
// over its storage capacity
func WithAccountStorageLimit(enabled bool) Option {
	return func(ctx Context) Context {
		ctx.LimitAccountStorage = enabled
		return ctx
	}
}

// WithAuthorizationCheckxEnabled enables or disables pre-execution
// authorization checks.
func WithAuthorizationChecksEnabled(enabled bool) Option {
	return func(ctx Context) Context {
		ctx.AuthorizationChecksEnabled = enabled
		return ctx
	}
}

// WithSequenceNumberCheckAndIncrementEnabled enables or disables pre-execution
// sequence number check / increment.
func WithSequenceNumberCheckAndIncrementEnabled(enabled bool) Option {
	return func(ctx Context) Context {
		ctx.SequenceNumberCheckAndIncrementEnabled = enabled
		return ctx
	}
}

// WithAccountKeyWeightThreshold sets the key weight threshold used for
// authorization checks.  If the threshold is a negative number, signature
// verification is skipped.
//
// Note: This is set only by tests
func WithAccountKeyWeightThreshold(threshold int) Option {
	return func(ctx Context) Context {
		ctx.AccountKeyWeightThreshold = threshold
		return ctx
	}
}

// WithTransactionBodyExecutionEnabled enables or disables the transaction body
// execution.
//
// Note: This is disabled only by tests
func WithTransactionBodyExecutionEnabled(enabled bool) Option {
	return func(ctx Context) Context {
		ctx.TransactionBodyExecutionEnabled = enabled
		return ctx
	}
}

// WithTransactionFeesEnabled enables or disables deduction of transaction fees
func WithTransactionFeesEnabled(enabled bool) Option {
	return func(ctx Context) Context {
		ctx.TransactionFeesEnabled = enabled
		return ctx
	}
}

// WithRandomSourceHistoryCallAllowed enables or disables calling the `entropy` function
// within cadence
func WithRandomSourceHistoryCallAllowed(allowed bool) Option {
	return func(ctx Context) Context {
		ctx.RandomSourceHistoryCallAllowed = allowed
		return ctx
	}
}

// WithReusableCadenceRuntimePool set the (shared) RedusableCadenceRuntimePool
// use for creating the cadence runtime.
func WithReusableCadenceRuntimePool(
	pool reusableRuntime.ReusableCadenceRuntimePool,
) Option {
	return func(ctx Context) Context {
		ctx.ReusableCadenceRuntimePool = pool
		return ctx
	}
}

// WithDerivedBlockData sets the derived data cache storage to be used by the
// transaction/script.
func WithDerivedBlockData(derivedBlockData *derived.DerivedBlockData) Option {
	return func(ctx Context) Context {
		ctx.DerivedBlockData = derivedBlockData
		return ctx
	}
}

// WithEventEncoder sets events encoder to be used for encoding events emitted during execution
func WithEventEncoder(encoder environment.EventEncoder) Option {
	return func(ctx Context) Context {
		ctx.EventEncoder = encoder
		return ctx
	}
}

// WithEVMEnabled enables access to the evm environment
func WithEVMEnabled(enabled bool) Option {
	return func(ctx Context) Context {
		ctx.EVMEnabled = enabled
		return ctx
	}
}
