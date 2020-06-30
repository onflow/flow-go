package fvm

import (
	"github.com/onflow/cadence/runtime"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/hash"
)

func Script(script []byte) InvokableScript {
	return InvokableScript{
		script: script,
	}
}

type InvokableScript struct {
	script []byte
	args   [][]byte
}

func (i InvokableScript) WithArguments(args [][]byte) InvokableScript {
	return InvokableScript{
		script: i.script,
		args:   args,
	}
}

func (i InvokableScript) Parse(vm *VirtualMachine, ctx Context, ledger Ledger) (Invokable, error) {
	panic("implement me")
}

func (i InvokableScript) Invoke(vm *VirtualMachine, ctx Context, ledger Ledger) (*InvocationResult, error) {
	env := newEnvironment(vm, ctx, ledger)

	scriptHash := hash.DefaultHasher.ComputeHash(i.script)
	location := runtime.ScriptLocation(scriptHash)

	scriptID := flow.HashToID(scriptHash)

	value, err := vm.runtime.ExecuteScript(i.script, i.args, env, location)

	return createInvocationResult(scriptID, value, env.getEvents(), env.getLogs(), err)
}
