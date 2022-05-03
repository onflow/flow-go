package debug_test

import (
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/debug"
)

func TestDebugger_RunTransaction(t *testing.T) {

	grpcAddress := "35.208.127.205:9000"
	chain := flow.Testnet.Chain()
	debugger := debug.NewRemoteDebugger(grpcAddress, chain, zerolog.New(os.Stdout).With().Logger())

	const scriptTemplate = `
	transaction(name: String) {
		prepare(signer: AuthAccount) {
			signer.contracts.remove(name: name)
		}
	}
	`

	offendingAddress := flow.HexToAddress("0x39543b318ac31299")

	script := []byte(scriptTemplate)
	txBody := flow.NewTransactionBody().
		SetGasLimit(9999).
		SetScript(script).
		AddArgument(jsoncdc.MustEncode(cadence.String("CheezeMarket"))).
		SetPayer(offendingAddress).
		SetProposalKey(offendingAddress, 0, 8168)
	txBody.Authorizers = []flow.Address{offendingAddress}

	// Run at the latest blockID
	//txErr, err := debugger.RunTransaction(txBody)
	//require.NoError(t, txErr)
	//require.NoError(t, err)

	// Run with blockID (use the file cache)
	blockId, err := flow.HexStringToIdentifier("0599498a64784b2be2afb854032e12b6428453165a1d21812cabd2b54f995c30")
	require.NoError(t, err)
	//txErr, err = debugger.RunTransactionAtBlockID(txBody, blockId, "")
	//require.NoError(t, txErr)
	//require.NoError(t, err)

	testCacheFile := "test.cache"
	// defer os.Remove(testCacheFile)
	// the first run would cache the results
	txErr, err := debugger.RunTransactionAtBlockID(txBody, blockId, testCacheFile)
	require.NoError(t, txErr)
	require.NoError(t, err)

	// second one should only use the cache
	// make blockId invalid so if it endsup looking up by id it should fail
	//blockId = flow.Identifier{}
	//txErr, err = debugger.RunTransactionAtBlockID(txBody, blockId, testCacheFile)
	//require.NoError(t, txErr)
	//require.NoError(t, err)
}
