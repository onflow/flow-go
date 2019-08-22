package core

import (
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime"
	"github.com/dapperlabs/bamboo-node/pkg/types"

	eruntime "github.com/dapperlabs/bamboo-node/internal/emulator/runtime"
	etypes "github.com/dapperlabs/bamboo-node/internal/emulator/types"
)

// Computer provides an interface to execute scripts against the world state.
type Computer struct {
	runtime runtime.Runtime
}

// NewComputer returns a new computer instance.
func NewComputer(runtime runtime.Runtime) *Computer {
	return &Computer{
		runtime: runtime,
	}
}

// ExecuteTransaction executes a transaction against the current world state.
func (c *Computer) ExecuteTransaction(
	tx *types.SignedTransaction,
	registers *etypes.RegistersView,
) (err error) {
	// TODO: deduct gas cost from transaction signer's account

	// TODO: more signatures
	accounts := []types.Address{
		tx.PayerSignature.Account,
	}

	inter := eruntime.NewEmulatorRuntimeAPI(registers)
	inter.Accounts = accounts
	_, err = c.ExecuteScript(tx.Script, inter)
	return err
}

// ExecuteScript executes a script against the current world state.
func (c *Computer) ExecuteScript(
	script []byte,
	inter runtime.Interface,
) (result interface{}, err error) {
	return c.runtime.ExecuteScript(script, inter)
}
