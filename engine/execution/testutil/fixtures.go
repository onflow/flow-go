package testutil

import (
	"encoding/hex"
	"fmt"

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
		Authorizers: []flow.Address{flow.RootAddress},
	}
}

func CreateCounterTransaction() flow.TransactionBody {
	return flow.TransactionBody{
		Script: []byte(`

			import 0x01

			transaction {
				prepare(acc: AuthAccount) {
					if acc.storage[Container.Counter] == nil {
                		let existing <- acc.storage[Container.Counter] <- Container.createCounter(3)
                		destroy existing
					}
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
					acc.storage[Container.Counter].add(2)
              	}
            }`),
		Authorizers: []flow.Address{flow.RootAddress},
	}
}
