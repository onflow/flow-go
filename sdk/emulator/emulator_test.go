package emulator_test

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

const counterScript = `
    pub resource Counter {
        pub var count: Int

        init() {
            self.count = 0
        }

        pub fun add(_ count: Int) {
            self.count = self.count + count
        }
    }

    pub fun createCounter(): <-Counter {
        return <-create Counter()
    }
`

// generateAddTwoToCounterScript generates a script that increments a counter.
// If no counter exists, it is created.
func generateAddTwoToCounterScript(counterAddress flow.Address) string {
	return fmt.Sprintf(
		`
		    import 0x%s

			transaction {

			  prepare(signer: Account) {
    	        if signer.storage[Counter] == nil {
    	            let existing <- signer.storage[Counter] <- createCounter()
    	            destroy existing
                    signer.published[&Counter] = &signer.storage[Counter] as Counter
    	        }
				signer.published[&Counter]?.add(2)
			  }
			}
		`,
		counterAddress,
	)
}

func generateGetCounterCountScript(counterAddress flow.Address, accountAddress flow.Address) string {
	return fmt.Sprintf(
		`
		    import 0x%s

			pub fun main(): Int {
                return getAccount(0x%s).published[&Counter]?.count ?? 0
			}
		`,
		counterAddress,
		accountAddress,
	)
}

// Returns a nonce value that is guaranteed to be unique.
var getNonce = func() func() uint64 {
	var nonce uint64
	return func() uint64 {
		nonce++
		return nonce
	}
}()
