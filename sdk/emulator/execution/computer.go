package execution

import (
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime"
	"github.com/dapperlabs/bamboo-node/pkg/types"
)

type Computer struct {
	runtime       runtime.Runtime
	runtimeLogger func(string)
}

func NewComputer(runtime runtime.Runtime, runtimeLogger func(string)) *Computer {
	return &Computer{
		runtime:       runtime,
		runtimeLogger: runtimeLogger,
	}
}

func (c *Computer) ExecuteTransaction(registers *RegistersView, tx *types.SignedTransaction) error {
	runtimeContext := NewRuntimeContext(registers)
	runtimeContext.Accounts = []types.Address{tx.PayerSignature.Account}
	runtimeContext.Logger = c.runtimeLogger

	_, err := c.runtime.ExecuteScript(tx.Script, runtimeContext)
	return err
}

func (c *Computer) ExecuteScript(registers *RegistersView, script []byte) (interface{}, error) {
	runtimeContext := NewRuntimeContext(registers)
	runtimeContext.Logger = c.runtimeLogger

	return c.runtime.ExecuteScript(script, runtimeContext)
}
