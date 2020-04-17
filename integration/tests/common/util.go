package common

import (
	"bytes"
	"context"
	"fmt"

	encoding "github.com/dapperlabs/cadence/encoding/xdr"

	"github.com/dapperlabs/flow-go/engine/ghost/client"
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/dsl"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// deployCounter deploys a counter contract using the given client.
func deployCounter(ctx context.Context, client *testnet.Client) error {
	return client.DeployContract(ctx, unittest.ContractCounterFixture())
}

// readCounter executes a script to read the value of a counter. The counter
// must have been deployed and created.
func readCounter(ctx context.Context, client *testnet.Client) (int, error) {

	script := dsl.Main{
		ReturnType: "Int",
		Code:       "return getAccount(0x01).published[&Testing.Counter]?.count ?? -3",
	}

	res, err := client.ExecuteScript(ctx, script)
	if err != nil {
		return 0, err
	}

	decoder := encoding.NewDecoder(bytes.NewReader(res))
	i, err := decoder.DecodeInt()
	if err != nil {
		return 0, err
	}

	return int(i.Value.Int64()), nil
}

// createCounter creates a counter instance in the root account. The counter
// contract must first have been deployed.
func createCounter(ctx context.Context, client *testnet.Client) error {

	txDSL := dsl.Transaction{
		Import: dsl.Import{Address: flow.RootAddress},
		Content: dsl.Prepare{
			Content: dsl.Code(`
				if signer.storage[Testing.Counter] == nil {
				let existing <- signer.storage[Testing.Counter] <- Testing.createCounter()
            	    destroy existing
            	    signer.published[&Testing.Counter] = &signer.storage[Testing.Counter] as Testing.Counter
            	}
            	signer.published[&Testing.Counter]?.add(2)`),
		},
	}

	tx := unittest.TransactionBodyFixture(unittest.WithTransactionDSL(txDSL))
	return client.SendTransaction(ctx, tx)
}

func GetGhostClient(ghostContainer *testnet.Container) (*client.GhostClient, error) {

	if !ghostContainer.Config.Ghost {
		return nil, fmt.Errorf("container is a not a ghost node container")
	}

	ghostPort, ok := ghostContainer.Ports[testnet.GhostNodeAPIPort]
	if !ok {
		return nil, fmt.Errorf("ghost node API port not found")
	}

	addr := fmt.Sprintf(":%s", ghostPort)

	return client.NewGhostClient(addr)
}
