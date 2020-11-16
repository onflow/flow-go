package reporters

import (
	"github.com/onflow/flow-go/ledger"
)

func AccountReporter(logger, payloads []ledger.Payload) error {

	accounts := make(map[string]bool)
	for _, p := range payloads {
		// owner
		owner := p.Key.KeyParts[0].Value
		accounts[string(owner)] = true
	}

	// number of accounts
	// accounts with most storage
	// median storage used
	return nil
}

func FlowTokenReporter(payloads []ledger.Payload) error {
	return nil
}
