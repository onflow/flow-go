package fvm

import (
	"github.com/onflow/cadence/runtime"

	"github.com/dapperlabs/flow-go/model/flow"
)

// A Context is an execution context in which invokables are applied to the
// ledger state.
type Context interface {
	NewChild(opts ...Option) Context

	Parse(i Invokable, ledger Ledger) (Invokable, error)
	Invoke(i Invokable, ledger Ledger) (*InvocationResult, error)
	GetAccount(address flow.Address, ledger Ledger) (*flow.Account, error)

	Options() Options
	Runtime() runtime.Runtime
}

type context struct {
	rt   runtime.Runtime
	opts Options
}

func newContext(rt runtime.Runtime, options Options, opts ...Option) Context {
	for _, applyOption := range opts {
		options = applyOption(options)
	}

	return &context{
		rt:   rt,
		opts: options,
	}
}

func (ctx *context) NewChild(opts ...Option) Context {
	return newContext(ctx.rt, ctx.opts, opts...)
}

func (ctx *context) Parse(i Invokable, ledger Ledger) (Invokable, error) {
	return i.Parse(ctx, ledger)
}

func (ctx *context) Invoke(i Invokable, ledger Ledger) (*InvocationResult, error) {
	return i.Invoke(ctx, ledger)
}

func (ctx *context) GetAccount(address flow.Address, ledger Ledger) (*flow.Account, error) {
	account, err := getAccount(ctx, ledger, ctx.opts.Chain, address)
	if err != nil {
		// TODO: wrap error
		return nil, err
	}

	return account, nil
}

func (ctx *context) Options() Options {
	return ctx.opts
}

func (ctx *context) Runtime() runtime.Runtime {
	return ctx.rt
}
