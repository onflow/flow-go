package common

import (
	"github.com/dapperlabs/flow-go/ledger"
)

// PayloadsToValues extracts values from an slice of payload
func PayloadsToValues(payloads []ledger.Payload) ([]ledger.Value, error) {
	ret := make([]ledger.Value, 0)
	for _, p := range payloads {
		ret = append(ret, p.Value)
	}
	return ret, nil
}

// UpdateToPayloads constructs an slice of payloads given ledger update
func UpdateToPayloads(update *ledger.Update) ([]ledger.Payload, error) {
	keys := update.Keys()
	values := update.Values()
	payloads := make([]ledger.Payload, 0)
	for i := range keys {
		payloads = append(payloads, ledger.Payload{Key: keys[i], Value: values[i]})
	}
	return payloads, nil
}
