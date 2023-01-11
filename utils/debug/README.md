

## Remote Debugger 

Remote debugger provides utils needed to run transactions and scripts against live network data. It uses GRPC endpoints on an execution nodes to fetch registers and block info when running a transaction. This is mostly provided for debugging purpose and should not be used for production level operations. 
If you use the caching method you can run the transaction once and use the cached values to run transaction in debugging mode. 

### sample code 

```GO
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

func TestDebugger_RunTransaction(t *testing.T) {

	grpcAddress := "localhost:3600"
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

	// Run with blockID (use the file cache)
	blockId, err := flow.HexStringToIdentifier("3a8281395e2c1aaa3b8643d148594b19e2acb477611a8e0cab8a55c46c40b563")
	require.NoError(t, err)
	txErr, err = debugger.RunTransactionAtBlockID(txBody, blockId, "")
	require.NoError(t, txErr)
	require.NoError(t, err)

	testCacheFile := "test.cache"
	defer os.Remove(testCacheFile)
	// the first run would cache the results
	txErr, err = debugger.RunTransactionAtBlockID(txBody, blockId, testCacheFile)
	require.NoError(t, txErr)
	require.NoError(t, err)

	// second one should only use the cache
	// make blockId invalid so if it endsup looking up by id it should fail
	blockId = flow.Identifier{}
	txErr, err = debugger.RunTransactionAtBlockID(txBody, blockId, testCacheFile)
	require.NoError(t, txErr)
	require.NoError(t, err)
}


```