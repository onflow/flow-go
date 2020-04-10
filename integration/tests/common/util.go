package common

import (
	"bytes"
	"context"

	encoding "github.com/dapperlabs/cadence/encoding/xdr"

	"github.com/dapperlabs/flow-go/integration/dsl"
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

var (
	CounterOwner      = "0x01"
	CounterController = "Testing.Counter"
	CounterKey        = "count"
)

// deployCounter deploys a counter contract using the given client.
func DeployCounter(ctx context.Context, client *testnet.Client) error {

	contract := dsl.Contract{
		Name: "Testing",
		Members: []dsl.CadenceCode{
			dsl.Resource{
				Name: "Counter",
				Code: `
				pub var count: Int

				init() {
					self.count = 0
				}
				pub fun add(_ count: Int) {
					self.count = self.count + count
				}`,
			},
			dsl.Code(`
				pub fun createCounter(): @Counter {
					return <-create Counter()
				}`,
			),
		},
	}

	return client.DeployContract(ctx, contract)
}

// readCounter executes a script to read the value of a counter. The counter
// must have been deployed and created.
func ReadCounter(ctx context.Context, client *testnet.Client) (int, error) {

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
func CreateCounter(ctx context.Context, client *testnet.Client) error {

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
