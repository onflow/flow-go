package fvm

import (
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

func Account(address flow.Address, chain flow.Chain) *FetchAccountProcedure {
	return &FetchAccountProcedure{
		accountFetcher: process.NewAccountFetcher()
		address: address,
		chain:   chain,
		Account: nil,
		Err: nil,
	}
}

type FetchAccountProcedure struct {
	accountFetcher: fvm.Processor
	address flow.Address
	chain flow.Chain
	Account *flow.Account
	Err     Error
}

func (proc *FetchAccountProcedure) Address() flow.Chain{
	return proc.address
}

func (proc *FetchAccountProcedure) Chain() flow.Chain{
	return proc.chain
}

func (proc *FetchAccountProcedure) Run(vm VirtualMachine, ctx Context, st *state.State) error {

	env, err := fvm.NewEnvironment(ctx, st)
	if err != nil {
		return err
	}

	return proc.accountFetcher.Process(vm, proc, env)
}
