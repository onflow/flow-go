package fvm

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type Options struct {
	astCache                         ASTCache
	blocks                           Blocks
	gasLimit                         uint64
	blockHeader                      *flow.Header
	signatureVerificationEnabled     bool
	feePaymentsEnabled               bool
	restrictedAccountCreationEnabled bool
	restrictedDeploymentEnabled      bool
}

const defaultGasLimit = 100000

func defaultOptions() Options {
	return Options{
		astCache:                         nil,
		gasLimit:                         defaultGasLimit,
		blockHeader:                      nil,
		signatureVerificationEnabled:     true,
		feePaymentsEnabled:               true,
		restrictedAccountCreationEnabled: true,
		restrictedDeploymentEnabled:      true,
	}
}

type Option func(config Options) Options

func WithCache(cache ASTCache) Option {
	return func(opts Options) Options {
		opts.astCache = cache
		return opts
	}
}

func WithGasLimit(limit uint64) Option {
	return func(opts Options) Options {
		opts.gasLimit = limit
		return opts
	}
}

func WithBlockHeader(header *flow.Header) Option {
	return func(opts Options) Options {
		opts.blockHeader = header
		return opts
	}
}

func WithBlocks(blocks Blocks) Option {
	return func(opts Options) Options {
		opts.blocks = blocks
		return opts
	}
}

func WithSignatureVerification(enabled bool) Option {
	return func(opts Options) Options {
		opts.signatureVerificationEnabled = enabled
		return opts
	}
}

func WithFeePayments(enabled bool) Option {
	return func(opts Options) Options {
		opts.feePaymentsEnabled = enabled
		return opts
	}
}

func WithRestrictedDeployment(enabled bool) Option {
	return func(opts Options) Options {
		opts.restrictedDeploymentEnabled = enabled
		return opts
	}
}

func WithRestrictedAccountCreation(enabled bool) Option {
	return func(opts Options) Options {
		opts.restrictedAccountCreationEnabled = enabled
		return opts
	}
}
