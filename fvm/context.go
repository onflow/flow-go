package fvm

import (
	"github.com/onflow/cadence/runtime/sema"
	"math"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/handler"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// A Context defines a set of execution parameters used by the virtual machine.
type Context struct {
	Chain   flow.Chain
	Blocks  Blocks
	Metrics handler.MetricsReporter
	Tracer  module.Tracer
	// AllowContextOverrideByExecutionState is a flag telling the fvm to override certain parts of the context from the state
	AllowContextOverrideByExecutionState bool
	ComputationLimit                     uint64
	MemoryLimit                          uint64
	MaxStateKeySize                      uint64
	MaxStateValueSize                    uint64
	MaxStateInteractionSize              uint64
	EventCollectionByteSizeLimit         uint64
	MaxNumOfTxRetries                    uint8
	BlockHeader                          *flow.Header
	ServiceAccountEnabled                bool
	// Deprecated: RestrictedDeploymentEnabled is deprecated use SetIsContractDeploymentRestrictedTransaction instead.
	// Can be removed after all networks are migrated to SetIsContractDeploymentRestrictedTransaction
	RestrictContractDeployment    bool
	RestrictContractRemoval       bool
	LimitAccountStorage           bool
	TransactionFeesEnabled        bool
	CadenceLoggingEnabled         bool
	EventCollectionEnabled        bool
	ServiceEventCollectionEnabled bool
	AccountFreezeEnabled          bool
	ExtensiveTracing              bool
	TransactionProcessors         []TransactionProcessor
	ScriptProcessors              []ScriptProcessor
	Logger                        zerolog.Logger
}

func (c Context) CheckerOptions() []sema.Option {

	if c.BlockHeader != nil {
		chainID := c.Chain.ChainID()
		blockHeight := c.BlockHeader.Height

		// backwards compatibility to enable rolling deploy
		if (chainID == flow.Mainnet && blockHeight > 1) ||
			(chainID == flow.Testnet && blockHeight > 1) {

			return nil
		}
	}

	return []sema.Option{
		sema.WithAllowResourceInvalidationAfterPotentialJump(true),
	}
}

// NewContext initializes a new execution context with the provided options.
func NewContext(logger zerolog.Logger, opts ...Option) Context {
	return newContext(defaultContext(logger), opts...)
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

const AccountKeyWeightThreshold = 1000

const (
	DefaultComputationLimit             = 100_000        // 100K
	DefaultMemoryLimit                  = math.MaxUint64 //
	DefaultEventCollectionByteSizeLimit = 256_000        // 256KB
	DefaultMaxNumOfTxRetries            = 3
)

func defaultContext(logger zerolog.Logger) Context {
	return Context{
		Chain:                                flow.Mainnet.Chain(),
		Blocks:                               nil,
		Metrics:                              &handler.NoopMetricsReporter{},
		Tracer:                               nil,
		AllowContextOverrideByExecutionState: true,
		ComputationLimit:                     DefaultComputationLimit,
		MemoryLimit:                          DefaultMemoryLimit,
		MaxStateKeySize:                      state.DefaultMaxKeySize,
		MaxStateValueSize:                    state.DefaultMaxValueSize,
		MaxStateInteractionSize:              state.DefaultMaxInteractionSize,
		EventCollectionByteSizeLimit:         DefaultEventCollectionByteSizeLimit,
		MaxNumOfTxRetries:                    DefaultMaxNumOfTxRetries,
		BlockHeader:                          nil,
		ServiceAccountEnabled:                true,
		RestrictContractDeployment:           true,
		RestrictContractRemoval:              true,
		CadenceLoggingEnabled:                false,
		EventCollectionEnabled:               true,
		ServiceEventCollectionEnabled:        false,
		AccountFreezeEnabled:                 true,
		ExtensiveTracing:                     false,
		TransactionProcessors: []TransactionProcessor{
			NewTransactionVerifier(AccountKeyWeightThreshold),
			NewTransactionSequenceNumberChecker(),
			NewTransactionInvoker(logger),
		},
		ScriptProcessors: []ScriptProcessor{
			NewScriptInvoker(),
		},
		Logger: logger,
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

// WithAllowContextOverrideByExecutionState sets if certain context parameters get loaded from the state or not
func WithAllowContextOverrideByExecutionState(load bool) Option {
	return func(ctx Context) Context {
		ctx.AllowContextOverrideByExecutionState = load
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

// WithBlockHeader sets the block header for a virtual machine context.
//
// The VM uses the header to provide current block information to the Cadence runtime,
// as well as to seed the pseudorandom number generator.
func WithBlockHeader(header *flow.Header) Option {
	return func(ctx Context) Context {
		ctx.BlockHeader = header
		return ctx
	}
}

// WithAccountFreezeEnabled enable/disable of account freeze functionality for a virtual machine context.
//
// With this option set to true, a setAccountFreeze function will be enabled for transactions processed by the VM
func WithAccountFreezeEnabled(accountFreezeEnabled bool) Option {
	return func(ctx Context) Context {
		ctx.AccountFreezeEnabled = accountFreezeEnabled
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

// WithExtensiveTracing sets the extensive tracing
func WithExtensiveTracing() Option {
	return func(ctx Context) Context {
		ctx.ExtensiveTracing = true
		return ctx
	}
}

// WithBlocks sets the block storage provider for a virtual machine context.
//
// The VM uses the block storage provider to provide historical block information to
// the Cadence runtime.
func WithBlocks(blocks Blocks) Option {
	return func(ctx Context) Context {
		ctx.Blocks = blocks
		return ctx
	}
}

// WithMetricsReporter sets the metrics collector for a virtual machine context.
//
// A metrics collector is used to gather metrics reported by the Cadence runtime.
func WithMetricsReporter(mr handler.MetricsReporter) Option {
	return func(ctx Context) Context {
		if mr != nil {
			ctx.Metrics = mr
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

// WithTransactionProcessors sets the transaction processors for a
// virtual machine context.
func WithTransactionProcessors(processors ...TransactionProcessor) Option {
	return func(ctx Context) Context {
		ctx.TransactionProcessors = processors
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

// @Depricated please use WithContractDeploymentRestricted instead of this
// this has been kept to reduce breaking change on the emulator, but would be
// removed at some point.
func WithRestrictedDeployment(restricted bool) Option {
	return WithContractDeploymentRestricted(restricted)
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

// WithTransactionFeesEnabled enables or disables deduction of transaction fees
func WithTransactionFeesEnabled(enabled bool) Option {
	return func(ctx Context) Context {
		ctx.TransactionFeesEnabled = enabled
		return ctx
	}
}
