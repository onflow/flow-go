package testutil

import (
	"encoding/hex"
	"fmt"

	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/model/flow"
)

func DeployCounterContractTransaction() flow.TransactionBody {
	encoded := hex.EncodeToString([]byte(`
			access(all) contract Container {
				access(all) resource Counter {
					pub var count: Int

					init(_ v: Int) {
						self.count = v
					}
					pub fun add(_ count: Int) {
						self.count = self.count + count
					}
				}
				pub fun createCounter(_ v: Int): @Counter {
					return <-create Counter(v)
				}
			}`))

	return flow.TransactionBody{
		Script: []byte(fmt.Sprintf(`transaction {
              prepare(signer: AuthAccount) {
                signer.setCode("%s".decodeHex())
              }
            }`, encoded)),
		Authorizers: []flow.Address{flow.ServiceAddress},
	}
}

func CreateCounterTransaction() flow.TransactionBody {
	return flow.TransactionBody{
		Script: []byte(`
			import 0x01

			transaction {
				prepare(acc: AuthAccount) {
					var maybeCounter <- acc.load<@Container.Counter>(from: /storage/counter)

					if maybeCounter == nil {
						maybeCounter <-! Container.createCounter(3)
					}

					acc.save(<-maybeCounter!, to: /storage/counter)
				}
			}`),
		Authorizers: []flow.Address{flow.ServiceAddress},
	}
}

// CreateCounterPanicTransaction returns a transaction that will manipulate state by writing a new counter into storage
// and then panic. It can be used to test whether execution state stays untouched/will revert
func CreateCounterPanicTransaction() flow.TransactionBody {
	return flow.TransactionBody{
		Script: []byte(`
			import 0x01

			transaction {
				prepare(acc: AuthAccount) {
					if let existing <- acc.load<@Container.Counter>(from: /storage/counter) {
						destroy existing
            		}

					panic("fail for testing purposes")
              	}
            }`),
		Authorizers: []flow.Address{flow.ServiceAddress},
	}
}

func AddToCounterTransaction() flow.TransactionBody {
	return flow.TransactionBody{
		Script: []byte(`
			import 0x01

			transaction {
				prepare(acc: AuthAccount) {
					let counter = acc.borrow<&Container.Counter>(from: /storage/counter)
					counter?.add(2)
				}
			}`),
		Authorizers: []flow.Address{flow.ServiceAddress},
	}
}

func SignTransactionByRoot(tx *flow.TransactionBody, seqNum uint64) error {

	hasher, err := hash.NewHasher(flow.RootAccountPrivateKey.HashAlgo)
	if err != nil {
		return fmt.Errorf("cannot create hasher: %w", err)
	}

	err = tx.SetPayer(flow.ServiceAddress).
		SetProposalKey(flow.ServiceAddress, 0, seqNum).
		SignEnvelope(flow.ServiceAddress, 0, flow.RootAccountPrivateKey.PrivateKey, hasher)

	if err != nil {
		return fmt.Errorf("cannot sign tx: %w", err)
	}

	return nil
}

func RootBootstrappedLedger() (virtualmachine.Ledger, error) {
	ledger := make(virtualmachine.MapLedger)

	return ledger, BootstrapLedgerWithRootAccount(ledger)
}

func BootstrapLedgerWithRootAccount(ledger virtualmachine.Ledger) error {

	ledgerAccess := virtualmachine.LedgerDAL{Ledger: ledger}

	rootAccountPublicKey := flow.RootAccountPrivateKey.PublicKey(virtualmachine.AccountKeyWeightThreshold)

	_, err := ledgerAccess.CreateAccountInLedger([]flow.AccountPublicKey{rootAccountPublicKey})
	if err != nil {
		return err
	}

	return nil
}
