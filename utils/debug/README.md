
## Remote Debugger 

The remote debugger allows running transactions and scripts against existing network data. 

It uses APIs to fetch registers and block info, for example the register value API of the execution nodes,
or the execution data API of the access nodes.

This is mostly provided for debugging purpose and should not be used for production level operations. 

Optionally, the debugger allows the fetched registers that are necessary for the execution to be written to
and read from a cache.

Use the `ExecutionNodeStorageSnapshot` to fetch the registers and block info from the execution node (live/recent data).

Use the `ExecutionDataStorageSnapshot` to fetch the execution data from the access node (recent/historic data).

### Sample Code

```GO
package debug_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/onflow/flow/protobuf/go/flow/executiondata"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/debug"
)

func getTransaction(chain flow.Chain) *flow.TransactionBody {

	const scriptTemplate = `
	import FlowServiceAccount from 0x%s
	transaction() {
		prepare(signer: &Account) {
			log(signer.balance)
		}
	  }
	`

	script := []byte(fmt.Sprintf(scriptTemplate, chain.ServiceAddress()))
	txBody := flow.NewTransactionBody().
		SetComputeLimit(9999).
		SetScript(script).
		SetPayer(chain.ServiceAddress()).
		SetProposalKey(chain.ServiceAddress(), 0, 0)
	txBody.Authorizers = []flow.Address{chain.ServiceAddress()}

	return txBody
}

func TestDebugger_RunTransactionAgainstExecutionNodeAtBlockID(t *testing.T) {

	host := "execution-001.mainnet26.nodes.onflow.org:9000"

	conn, err := grpc.NewClient(
		host,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	executionClient := execution.NewExecutionAPIClient(conn)

	blockID, err := flow.HexStringToIdentifier("e68a9a1fe849d1be80e4c5e414f53e3b59a170b88785e0b22be077ae9c3bbd29")
	require.NoError(t, err)

	header, err := debug.GetExecutionAPIBlockHeader(executionClient, context.Background(), blockID)

	snapshot, err := debug.NewExecutionNodeStorageSnapshot(executionClient, nil, blockID)
	require.NoError(t, err)

	defer snapshot.Close()

	chain := flow.Mainnet.Chain()
	logger := zerolog.New(os.Stdout).With().Logger()
	debugger := debug.NewRemoteDebugger(chain, logger)

	txBody := getTransaction(chain)

	txErr, err := debugger.RunTransaction(txBody, snapshot, header)
	require.NoError(t, txErr)
	require.NoError(t, err)
}

func TestDebugger_RunTransactionAgainstAccessNodeAtBlockIDWithFileCache(t *testing.T) {

	host := "access.mainnet.nodes.onflow.org:9000"

	conn, err := grpc.NewClient(
		host,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	executionDataClient := executiondata.NewExecutionDataAPIClient(conn)

	var blockHeight uint64 = 100_000_000
	
	blockID, err := flow.HexStringToIdentifier("e68a9a1fe849d1be80e4c5e414f53e3b59a170b88785e0b22be077ae9c3bbd29")
	require.NoError(t, err)

	accessClient := access.NewAccessAPIClient(conn)
	header, err := debug.GetAccessAPIBlockHeader(
		accessClient,
		context.Background(),
		blockID,
	)
	require.NoError(t, err)

	testCacheFile := "test.cache"
	defer os.Remove(testCacheFile)

	cache := debug.NewFileRegisterCache(testCacheFile)

	snapshot, err := debug.NewExecutionDataStorageSnapshot(executionDataClient, cache, blockHeight)
	require.NoError(t, err)

	defer snapshot.Close()

	chain := flow.Mainnet.Chain()
	logger := zerolog.New(os.Stdout).With().Logger()
	debugger := debug.NewRemoteDebugger(chain, logger)

	txBody := getTransaction(chain)

	// the first run will cache the results
	txErr, err := debugger.RunTransaction(txBody, snapshot, header)
	require.NoError(t, txErr)
	require.NoError(t, err)

	// the second run should only use the cache.
	txErr, err = debugger.RunTransaction(txBody, snapshot, header)
	require.NoError(t, txErr)
	require.NoError(t, err)
}

```
