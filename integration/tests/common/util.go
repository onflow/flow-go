package common

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/onflow/cadence"

	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/integration/convert"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/dsl"
	"github.com/onflow/flow-go/utils/unittest"
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
)

// CreateCounterTx is a transaction script for creating an instance of the counter in the account storage of the
// authorizing account NOTE: the counter contract must be deployed first
func CreateCounterTx(counterAddress sdk.Address) dsl.Transaction {
	return dsl.Transaction{
		Import: dsl.Import{Address: counterAddress},
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
}

// ReadCounterScript is a read-only script for reading the current value of the counter contract
func ReadCounterScript(contractAddress sdk.Address, accountAddress sdk.Address) dsl.Main {
	return dsl.Main{
		Import: dsl.Import{
			Names:   []string{"Testing"},
			Address: contractAddress,
		},
		ReturnType: "Int",
		Code: fmt.Sprintf(
			`
			  let account = getAccount(0x%s)
			  let cap = account.getCapability(/public/counter)
              return cap.borrow<&Testing.Counter>()?.count ?? -3
            `,
			accountAddress.Hex(),
		),
	}
}

// CreateCounterPanicTx is a transaction script that creates a counter instance in the root account, but panics after
// manipulating state. It can be used to test whether execution state stays untouched/will revert. NOTE: the counter
// contract must be deployed first
func CreateCounterPanicTx(chain flow.Chain) dsl.Transaction {
	return dsl.Transaction{
		Import: dsl.Import{Address: sdk.Address(chain.ServiceAddress())},
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
}

// ReadCounter executes a script to read the value of a counter. The counter
// must have been deployed and created.
func ReadCounter(ctx context.Context, client *testnet.Client, address sdk.Address) (int, error) {

	res, err := client.ExecuteScript(ctx, ReadCounterScript(address, address))
	if err != nil {
		return 0, err
	}

	return res.(cadence.Int).Int(), nil
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
func GetAccount(chain flow.Chain) (sdk.Address, *sdk.AccountKey, sdkcrypto.Signer) {

	addr := sdk.Address(chain.ServiceAddress())

	key := RandomPrivateKey()
	signer := sdkcrypto.NewInMemorySigner(key, sdkcrypto.SHA3_256)

	acct := sdk.NewAccountKey().
		FromPrivateKey(key).
		SetHashAlgo(sdkcrypto.SHA3_256).
		SetWeight(sdk.AccountKeyWeightThreshold)

	return addr, acct, signer
}

// RandomPrivateKey returns a randomly generated ECDSA P-256 private key.
func RandomPrivateKey() sdkcrypto.PrivateKey {
	seed := make([]byte, sdkcrypto.MinSeedLength)

	_, err := rand.Read(seed)
	if err != nil {
		panic(err)
	}

	privateKey, err := sdkcrypto.GeneratePrivateKey(sdkcrypto.ECDSA_P256, seed)
	if err != nil {
		panic(err)
	}

	return privateKey
}

func SDKTransactionFixture(opts ...func(*sdk.Transaction)) sdk.Transaction {
	tx := sdk.Transaction{
		Script:             []byte("pub fun main() {}"),
		ReferenceBlockID:   sdk.Identifier(unittest.IdentifierFixture()),
		GasLimit:           10,
		ProposalKey:        convert.ToSDKProposalKey(unittest.ProposalKeyFixture()),
		Payer:              sdk.Address(unittest.AddressFixture()),
		Authorizers:        []sdk.Address{sdk.Address(unittest.AddressFixture())},
		PayloadSignatures:  []sdk.TransactionSignature{},
		EnvelopeSignatures: []sdk.TransactionSignature{convert.ToSDKTransactionSignature(unittest.TransactionSignatureFixture())},
	}

	for _, apply := range opts {
		apply(&tx)
	}

	return tx
}

func WithTransactionDSL(txDSL dsl.Transaction) func(tx *sdk.Transaction) {
	return func(tx *sdk.Transaction) {
		tx.Script = []byte(txDSL.ToCadence())
	}
}

func WithReferenceBlock(id sdk.Identifier) func(tx *sdk.Transaction) {
	return func(tx *sdk.Transaction) {
		tx.ReferenceBlockID = id
	}
}

// WithChainID modifies the default fixture to use addresses consistent with the
// given chain ID.
func WithChainID(chainID flow.ChainID) func(tx *sdk.Transaction) {
	service := convert.ToSDKAddress(chainID.Chain().ServiceAddress())
	return func(tx *sdk.Transaction) {
		tx.Payer = service
		tx.Authorizers = []sdk.Address{service}

		tx.ProposalKey.Address = service
		for i, sig := range tx.PayloadSignatures {
			sig.Address = service
			tx.PayloadSignatures[i] = sig
		}
		for i, sig := range tx.EnvelopeSignatures {
			sig.Address = service
			tx.EnvelopeSignatures[i] = sig
		}
	}
}
