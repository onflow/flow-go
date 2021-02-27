package fvm

import (
	"github.com/onflow/cadence"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

// A Context defines a set of execution parameters used by the virtual machine.
type Context struct {
	Chain                            flow.Chain
	Blocks                           Blocks
	Metrics                          *MetricsCollector
	GasLimit                         uint64
	MaxStateKeySize                  uint64
	MaxStateValueSize                uint64
	MaxStateInteractionSize          uint64
	EventCollectionByteSizeLimit     uint64
	MaxNumOfTxRetries                uint8
	BlockHeader                      *flow.Header
	ServiceAccountEnabled            bool
	RestrictedAccountCreationEnabled bool
	RestrictedDeploymentEnabled      bool
	LimitAccountStorage              bool
	CadenceLoggingEnabled            bool
	SetValueHandler                  SetValueHandler
	SignatureVerifier                SignatureVerifier
	TransactionProcessors            []TransactionProcessor
	ScriptProcessors                 []ScriptProcessor
	Logger                           zerolog.Logger
}

// SetValueHandler receives a value written by the Cadence runtime.
type SetValueHandler func(owner flow.Address, key string, value cadence.Value) error

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
	DefaultGasLimit                     = 100_000 // 100K
	DefaultEventCollectionByteSizeLimit = 256_000 // 256KB
	DefaultMaxNumOfTxRetries            = 3
)

func defaultContext(logger zerolog.Logger) Context {
	return Context{
		Chain:                            flow.Mainnet.Chain(),
		Blocks:                           nil,
		Metrics:                          nil,
		GasLimit:                         DefaultGasLimit,
		MaxStateKeySize:                  state.DefaultMaxKeySize,
		MaxStateValueSize:                state.DefaultMaxValueSize,
		MaxStateInteractionSize:          state.DefaultMaxInteractionSize,
		EventCollectionByteSizeLimit:     DefaultEventCollectionByteSizeLimit,
		MaxNumOfTxRetries:                DefaultMaxNumOfTxRetries,
		BlockHeader:                      nil,
		ServiceAccountEnabled:            true,
		RestrictedAccountCreationEnabled: true,
		RestrictedDeploymentEnabled:      true,
		CadenceLoggingEnabled:            false,
		SetValueHandler:                  nil,
		SignatureVerifier:                NewDefaultSignatureVerifier(),
		TransactionProcessors: []TransactionProcessor{
			NewTransactionSignatureVerifier(AccountKeyWeightThreshold),
			NewTransactionSequenceNumberChecker(),
			NewTransactionFeeDeductor(),
			NewTransactionInvocator(logger),
			NewTransactionStorageLimiter(),
		},
		ScriptProcessors: []ScriptProcessor{
			NewScriptInvocator(),
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

// WithGasLimit sets the gas limit for a virtual machine context.
func WithGasLimit(limit uint64) Option {
	return func(ctx Context) Context {
		ctx.GasLimit = limit
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

// WithMetricsCollector sets the metrics collector for a virtual machine context.
//
// A metrics collector is used to gather metrics reported by the Cadence runtime.
func WithMetricsCollector(mc *MetricsCollector) Option {
	return func(ctx Context) Context {
		ctx.Metrics = mc
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

// WithRestrictedDeployment enables or disables restricted contract deployment for a
// virtual machine context.
func WithRestrictedDeployment(enabled bool) Option {
	return func(ctx Context) Context {
		ctx.RestrictedDeploymentEnabled = enabled
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

// WithRestrictedAccountCreation enables or disables restricted account creation for a
// virtual machine context
func WithRestrictedAccountCreation(enabled bool) Option {
	return func(ctx Context) Context {
		ctx.RestrictedAccountCreationEnabled = enabled
		return ctx
	}
}

// WithSetValueHandler sets a handler that is called when a value is written
// by the Cadence runtime.
func WithSetValueHandler(handler SetValueHandler) Option {
	return func(ctx Context) Context {
		ctx.SetValueHandler = handler
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
