package common

import (
	"bytes"
	"context"
	"fmt"

	encoding "github.com/onflow/cadence/encoding/xdr"
	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/examples"

	"github.com/dapperlabs/flow-go/engine/ghost/client"
	"github.com/dapperlabs/flow-go/integration/convert"
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/dsl"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

var (
	CounterOwner      = "0x01"
	CounterController = "Testing.Counter"
	CounterKey        = "count"
)

// DeployCounter deploys a counter contract using the given client.
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
func readCounter(ctx context.Context, client *testnet.Client) (int, error) {

	script := dsl.Main{
		ReturnType: "Int",
		Code: `
			let account = getAccount(0x01)
			if let cap = account.getCapability(/public/counter) {
				return cap.borrow<&Testing.Counter>()?.count ?? -3
			}
			return -3`,
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
				var maybeCounter <- signer.load<@Testing.Counter>(from: /storage/counter)

				if maybeCounter == nil {
					maybeCounter <-! Testing.createCounter()
				}

				maybeCounter?.add(2)
				signer.save(<-maybeCounter!, to: /storage/counter)

				signer.link<&Testing.Counter>(/public/counter, target: /storage/counter)
				`),
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

// GetAccount returns a new account address, key, and signer.
func GetAccount() (sdk.Address, *sdk.AccountKey, sdkcrypto.Signer) {

	addr := convert.ToSDKAddress(unittest.AddressFixture())

	key := examples.RandomPrivateKey()
	signer := sdkcrypto.NewInMemorySigner(key, sdkcrypto.SHA3_256)

	acct := sdk.NewAccountKey().
		FromPrivateKey(key).
		SetHashAlgo(sdkcrypto.SHA3_256).
		SetWeight(sdk.AccountKeyWeightThreshold)

	return addr, acct, signer
}
