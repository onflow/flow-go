package fvm

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Options defines the configuration parameters available for a virtual machine context.
type Options struct {
	Chain                            flow.Chain
	ASTCache                         ASTCache
	Blocks                           Blocks
	Metrics                          *MetricsCollector
	GasLimit                         uint64
	BlockHeader                      *flow.Header
	SignatureVerificationEnabled     bool
	FeePaymentsEnabled               bool
	RestrictedAccountCreationEnabled bool
	RestrictedDeploymentEnabled      bool
}

const defaultGasLimit = 100000

func defaultOptions(chain flow.Chain) Options {
	return Options{
		Chain:                            chain,
		ASTCache:                         nil,
		Blocks:                           nil,
		Metrics:                          nil,
		GasLimit:                         defaultGasLimit,
		BlockHeader:                      nil,
		SignatureVerificationEnabled:     true,
		FeePaymentsEnabled:               true,
		RestrictedAccountCreationEnabled: true,
		RestrictedDeploymentEnabled:      true,
	}
}

// An Option sets a configuration parameter for a virtual machine context.
type Option func(opts Options) Options

// WithASTCache sets the AST cache for a virtual machine context.
func WithASTCache(cache ASTCache) Option {
	return func(opts Options) Options {
		opts.ASTCache = cache
		return opts
	}
}

// WithGasLimit sets the gas limit for a virtual machine context.
func WithGasLimit(limit uint64) Option {
	return func(opts Options) Options {
		opts.GasLimit = limit
		return opts
	}
}

// WithBlockHeader sets the block header for a virtual machine context.
//
// The VM uses the header to provide current block information to the Cadence runtime,
// as well as to seed the pseudorandom number generator.
func WithBlockHeader(header *flow.Header) Option {
	return func(opts Options) Options {
		opts.BlockHeader = header
		return opts
	}
}

// WithBlocks sets the block storage provider for a virtual machine context.
//
// The VM uses the block storage provider to provide historical block information to
// the Cadence runtime.
func WithBlocks(blocks Blocks) Option {
	return func(opts Options) Options {
		opts.Blocks = blocks
		return opts
	}
}

// WithMetricsCollector sets the metrics collector for a virtual machine context.
//
// A metrics collector is used to gather metrics reported by the Cadence runtime.
func WithMetricsCollector(mc *MetricsCollector) Option {
	return func(opts Options) Options {
		opts.Metrics = mc
		return opts
	}
}

// WithSignatureVerification enables or disables signature verification and sequence
// number checks for a virtual machine context.
func WithSignatureVerification(enabled bool) Option {
	return func(opts Options) Options {
		opts.SignatureVerificationEnabled = enabled
		return opts
	}
}

// WithFeePayments enables or disables fee payments for a virtual machine context.
func WithFeePayments(enabled bool) Option {
	return func(opts Options) Options {
		opts.FeePaymentsEnabled = enabled
		return opts
	}
}

// WithRestrictedDeployment enables or disables restricted contract deployment for a
// virtual machine context.
func WithRestrictedDeployment(enabled bool) Option {
	return func(opts Options) Options {
		opts.RestrictedDeploymentEnabled = enabled
		return opts
	}
}

// WithRestrictedAccountCreation enables or disables restricted account creation for a
// virtual machine context
func WithRestrictedAccountCreation(enabled bool) Option {
	return func(opts Options) Options {
		opts.RestrictedAccountCreationEnabled = enabled
		return opts
	}
}
