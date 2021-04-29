package context

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/handler"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/trace"
)

// Runnable is anything that can be run by a virtual machine.
type Runnable interface {
	ID() flow.Identifier
	Script() []byte
	Arguments() [][]byte
	Run(vm *VirtualMachine, ctx Context, sth *state.StateHolder, programs *programs.Programs) error
}

type VirtualMachine interface {
	Runtime() runtime.Runtime
	Run(ctx Context, runnable Runnable, view state.View, programs *programs.Programs) (err error)
	Query(ctx Context, script []byte, view state.View, programs *programs.Programs) (cadence.Value, error)
	// TODO make GetAccount Special case of query
	GetAccount(ctx Context, address flow.Address, v state.View, programs *programs.Programs) (*flow.Account, error)
	InvokeMetaTransaction(ctx Context, metaTx *flow.TransactionBody, sth *state.StateHolder, programs *programs.Programs) (errors.Error, error)
}

type Blocks interface {
	// ByHeight returns the block at the given height
	ByHeightFrom(height uint64, header *flow.Header) (*flow.Header, error)
}

type Environment runtime.Interface

// A Context defines a set of execution parameters used by the virtual machine.
type Context struct {
	Chain                            flow.Chain
	Blocks                           Blocks
	BlockHeader                      *flow.Header
	Metrics                          handler.MetricsReporter
	Tracer                           module.Tracer
	GasLimit                         uint64
	MaxStateKeySize                  uint64
	MaxStateValueSize                uint64
	MaxStateInteractionSize          uint64
	EventCollectionByteSizeLimit     uint64
	MaxNumOfTxRetries                uint8
	ServiceAccountEnabled            bool
	RestrictedAccountCreationEnabled bool
	RestrictedDeploymentEnabled      bool
	LimitAccountStorage              bool
	CadenceLoggingEnabled            bool
	EventCollectionEnabled           bool
	ServiceEventCollectionEnabled    bool
	AccountFreezeAvailable           bool
	ExtensiveTracing                 bool
	AccountKeyWeightThreshold        int
	SetValueHandler                  SetValueHandler
	SignatureVerifier                crypto.SignatureVerifier // TODO maybe remove this as well.
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

const (
	DefaultAccountKeyWeightThreshold    = 1000
	DefaultGasLimit                     = 100_000 // 100K
	DefaultEventCollectionByteSizeLimit = 256_000 // 256KB
	DefaultMaxNumOfTxRetries            = 3
)

func defaultContext(logger zerolog.Logger) Context {
	return Context{
		Chain:                            flow.Mainnet.Chain(),
		Blocks:                           nil,
		Metrics:                          &handler.NoopMetricsReporter{},
		Tracer:                           trace.NewNoopTracer(),
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
		EventCollectionEnabled:           true,
		ServiceEventCollectionEnabled:    false,
		AccountFreezeAvailable:           false,
		ExtensiveTracing:                 false,
		SetValueHandler:                  nil,
		AccountKeyWeightThreshold:        DefaultAccountKeyWeightThreshold,
		SignatureVerifier:                crypto.NewDefaultSignatureVerifier(),
		Logger:                           logger,
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

// WithAccountFreezeAvailable sets availability of account freeze function for a virtual machine context.
//
// With this option set to true, a setAccountFreeze function will be enabled for transactions processed by the VM
func WithAccountFreezeAvailable(accountFreezeAvailable bool) Option {
	return func(ctx Context) Context {
		ctx.AccountFreezeAvailable = accountFreezeAvailable
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
