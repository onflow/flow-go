package testutil

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

const CounterContract = `
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
}
`

func DeployCounterContractTransaction(authorizer flow.Address) *flow.TransactionBody {
	return CreateContractDeploymentTransaction(CounterContract, authorizer)
}

func DeployUnauthorizedCounterContractTransaction(authorizer flow.Address) *flow.TransactionBody {
	return CreateUnauthorizedContractDeploymentTransaction(CounterContract, authorizer)
}

func CreateCounterTransaction(counter, signer flow.Address) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(`
			import 0x%s

			transaction {
				prepare(acc: AuthAccount) {
					var maybeCounter <- acc.load<@Container.Counter>(from: /storage/counter)

					if maybeCounter == nil {
						maybeCounter <-! Container.createCounter(3)
					}

					acc.save(<-maybeCounter!, to: /storage/counter)
				}
			}`, counter)),
		).
		AddAuthorizer(signer)
}

// CreateCounterPanicTransaction returns a transaction that will manipulate state by writing a new counter into storage
// and then panic. It can be used to test whether execution state stays untouched/will revert
func CreateCounterPanicTransaction(counter, signer flow.Address) *flow.TransactionBody {
	return &flow.TransactionBody{
		Script: []byte(fmt.Sprintf(`
			import 0x%s

			transaction {
				prepare(acc: AuthAccount) {
					if let existing <- acc.load<@Container.Counter>(from: /storage/counter) {
						destroy existing
            		}

					panic("fail for testing purposes")
              	}
            }`, counter)),
		Authorizers: []flow.Address{signer},
	}
}

func AddToCounterTransaction(counter, signer flow.Address) *flow.TransactionBody {
	return &flow.TransactionBody{
		Script: []byte(fmt.Sprintf(`
			import 0x%s

			transaction {
				prepare(acc: AuthAccount) {
					let counter = acc.borrow<&Container.Counter>(from: /storage/counter)
					counter?.add(2)
				}
			}`, counter)),
		Authorizers: []flow.Address{signer},
	}
}
