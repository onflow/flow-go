package testutil

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

func DeployCounterContractTransaction() flow.TransactionBody {
	return CreateContractDeploymentTransaction(`
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
		}`, flow.RootAddress)
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
		Authorizers: []flow.Address{flow.RootAddress},
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
		Authorizers: []flow.Address{flow.RootAddress},
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
		Authorizers: []flow.Address{flow.RootAddress},
	}
}
