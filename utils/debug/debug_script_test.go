package debug_test

import (
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/debug"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestDebugger_RunScript(t *testing.T) {

	grpcAddress := "xxxxxxx"
	chain := flow.Testnet.Chain()
	debugger := debug.NewRemoteDebugger(grpcAddress, chain, zerolog.New(os.Stdout).With().Logger())

	const scriptTemplate = `
		pub fun main(address: Address): [String] {
			let account = getAuthAccount(address)
			let paths: [String] = []
			account.forEachStored(fun (path: StoragePath, type: Type): Bool {
			  paths.append(path.toString())
			  return true
			})
			return paths
		}`

	arg, err := jsoncdc.Encode(cadence.NewAddress(flow.HexToAddress("0x7cf208094b12fdb8")))
	require.NoError(t, err)
	value, scriptErr, processErr := debugger.RunScript(
		[]byte(scriptTemplate),
		[][]byte{arg})

	require.NoError(t, scriptErr)
	require.NoError(t, processErr)
	require.NotNil(t, value)
}
