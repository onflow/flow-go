package testutil

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

const CounterContract = `
access(all) contract Container {
	access(all) resource Counter {
		access(all) var count: Int

		init(_ v: Int) {
			self.count = v
		}
		access(all) fun add(_ count: Int) {
			self.count = self.count + count
		}
	}
	access(all) fun createCounter(_ v: Int): @Counter {
		return <-create Counter(v)
	}
}
`

const CounterContractV2 = `
access(all) contract Container {
	access(all) resource Counter {
		access(all) var count: Int

		init(_ v: Int) {
			self.count = v
		}
		access(all) fun add(_ count: Int) {
			self.count = self.count + count
		}
	}
	access(all) fun createCounter(_ v: Int): @Counter {
		return <-create Counter(v)
	}

	access(all) fun createCounter2(_ v: Int): @Counter {
		return <-create Counter(v)
	}
}
`

func DeployCounterContractTransaction(authorizer flow.Address, chain flow.Chain) *flow.TransactionBody {
	return CreateContractDeploymentTransaction("Container", CounterContract, authorizer, chain)
}

func DeployUnauthorizedCounterContractTransaction(authorizer flow.Address) *flow.TransactionBody {
	return CreateUnauthorizedContractDeploymentTransaction("Container", CounterContract, authorizer)
}

func UpdateUnauthorizedCounterContractTransaction(authorizer flow.Address) *flow.TransactionBody {
	return UpdateContractUnathorizedDeploymentTransaction("Container", CounterContractV2, authorizer)
}

func RemoveUnauthorizedCounterContractTransaction(authorizer flow.Address) *flow.TransactionBody {
	return RemoveContractUnathorizedDeploymentTransaction("Container", authorizer)
}

func RemoveCounterContractTransaction(authorizer flow.Address, chain flow.Chain) *flow.TransactionBody {
	return RemoveContractDeploymentTransaction("Container", authorizer, chain)
}

func CreateCounterTransaction(counter, signer flow.Address) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(`
			import 0x%s

			transaction {
				prepare(acc: auth(Storage) &Account) {
					var maybeCounter <- acc.storage.load<@Container.Counter>(from: /storage/counter)

					if maybeCounter == nil {
						maybeCounter <-! Container.createCounter(3)
					}

					acc.storage.save(<-maybeCounter!, to: /storage/counter)
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
				prepare(acc: auth(Storage) &Account) {
					if let existing <- acc.storage.load<@Container.Counter>(from: /storage/counter) {
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
				prepare(acc: auth(Storage) &Account) {
					let counter = acc.storage.borrow<&Container.Counter>(from: /storage/counter)
					counter?.add(2)
				}
			}`, counter)),
		Authorizers: []flow.Address{signer},
	}
}
