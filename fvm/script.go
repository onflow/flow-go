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

func (i InvokableScript) Parse(ctx Context, ledger Ledger) (Invokable, error) {
	panic("implement me")
}

func (i InvokableScript) Invoke(ctx Context, ledger Ledger) (*InvocationResult, error) {
	env := ctx.NewEnvironment(ledger)

	scriptHash := hash.DefaultHasher.ComputeHash(i.script)
	location := runtime.ScriptLocation(scriptHash)

	scriptID := flow.HashToID(scriptHash)

	value, err := ctx.Runtime().ExecuteScript(i.script, env, location)

	return createInvocationResult(scriptID, value, env.getEvents(), env.getLogs(), err)
}
