package debug_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/debug"
)

func TestDebugger(t *testing.T) {

	// this code is mostly a sample code so we skip by default
	t.Skip()

	grpcAddress := "localhost:3569"
	chain := flow.Emulator.Chain()
	debugger := debug.NewRemoteDebugger(grpcAddress, chain, zerolog.New(os.Stdout).With().Logger())

	const scriptTemplate = `
	import FlowServiceAccount from 0x%s

	transaction() {
		prepare(signer: AuthAccount) {
			log(signer.balance)
		}
	  }
	`

	script := []byte(fmt.Sprintf(scriptTemplate, chain.ServiceAddress()))
	txBody := flow.NewTransactionBody().
		SetGasLimit(9999).
		SetScript([]byte(script)).
		SetPayer(chain.ServiceAddress()).
		SetProposalKey(chain.ServiceAddress(), 0, 0)
	txBody.Authorizers = []flow.Address{chain.ServiceAddress()}

	// Run at the latest blockID
	txErr, err := debugger.RunTransaction(txBody)
	require.NoError(t, txErr)
	require.NoError(t, err)

	// Run with blockID
	blockId, _ := flow.HexStringToIdentifier("93752591f84f90ca91c707413dfb0ce8ba346929fba66fe58ebd7f97f5f84f57")
	txErr, err = debugger.RunTransactionAtBlockID(txBody, blockId)
	require.NoError(t, txErr)
	require.NoError(t, err)

	t.Fatal("XXX")
}
