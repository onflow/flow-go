package common

import (
	"context"
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
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
	// CounterContract is a simple counter contract in Cadence
	CounterContract = dsl.Contract{
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

	// CreateCounterTx is a transaction script for creating an instance of the counter in the account storage of the
	// authorizing account NOTE: the counter contract must be deployed first
	CreateCounterTx = dsl.Transaction{
		Import: dsl.Import{Address: flow.ServiceAddress()},
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

	// ReadCounterScript is a read-only script for reading the current value of the counter contract
	ReadCounterScript = dsl.Main{
		Import: dsl.Import{
			Names:   []string{"Testing"},
			Address: flow.ServiceAddress()},
		ReturnType: "Int",
		Code: fmt.Sprintf(`
			let account = getAccount(0x%s)
			if let cap = account.getCapability(/public/counter) {
				return cap.borrow<&Testing.Counter>()?.count ?? -3
			}
			return -3`, flow.ServiceAddress().Short()),
	}

	// CreateCounterPanicTx is a transaction script that creates a counter instance in the root account, but panics after
	// manipulating state. It can be used to test whether execution state stays untouched/will revert. NOTE: the counter
	// contract must be deployed first
	CreateCounterPanicTx = dsl.Transaction{
		Import: dsl.Import{Address: flow.ServiceAddress()},
		Content: dsl.Prepare{
			Content: dsl.Code(`
				var maybeCounter <- signer.load<@Testing.Counter>(from: /storage/counter)

				if maybeCounter == nil {
					maybeCounter <-! Testing.createCounter()
				}

				maybeCounter?.add(2)
				signer.save(<-maybeCounter!, to: /storage/counter)

				signer.link<&Testing.Counter>(/public/counter, target: /storage/counter)

				panic("fail for testing purposes")
				`),
		},
	}
)

// readCounter executes a script to read the value of a counter. The counter
// must have been deployed and created.
func readCounter(ctx context.Context, client *testnet.Client) (int, error) {

	res, err := client.ExecuteScript(ctx, ReadCounterScript)
	if err != nil {
		return 0, err
	}

	v, err := jsoncdc.Decode(res)
	if err != nil {
		return 0, err
	}

	return v.(cadence.Int).Int(), nil
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
