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
