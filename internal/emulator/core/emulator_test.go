package core_test

import (
	"testing"
	"time"

	"github.com/dapperlabs/bamboo-node/internal/emulator/core"
	"github.com/dapperlabs/bamboo-node/internal/emulator/types"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

func TestSimulatedChain(t *testing.T) {
	b := core.NewEmulatedBlockchain()

	txA := &types.SignedTransaction{
		Transaction: &types.RawTransaction{
			Nonce: 16,
			Script: []byte(`
				fun main() {
					const controller = [1]
					const owner = [2]
					const key = [3]
					const value = getValue(controller, owner, key)
					setValue(controller, owner, key, value + 2)
				}
			`),
			ComputeLimit: 10,
			Timestamp:    time.Now(),
		},
		PayerSignature: crypto.Signature{},
	}

	b.SubmitTransaction(txA)
}
