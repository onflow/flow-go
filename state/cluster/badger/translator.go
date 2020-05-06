package badger

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	storage "github.com/dapperlabs/flow-go/storage/badger"
)

// Translator is a translation layer that determines the reference block on
// the main chain for a given cluster block, using the reference block from
// the cluster block's payload.
type Translator struct {
	payloads *storage.ClusterPayloads
}

// NewTranslator returns a new block ID translator.
func NewTranslator(payloads *storage.ClusterPayloads) *Translator {
	translator := &Translator{
		payloads: payloads,
	}
	return translator
}

// Translate retrieves the reference main-chain block ID for the given cluster
// block ID.
func (t *Translator) Translate(blockID flow.Identifier) (flow.Identifier, error) {

	payload, err := t.payloads.ByBlockID(blockID)
	if err != nil {
		return flow.ZeroID, fmt.Errorf("could not retrieve reference block payload: %w", err)
	}

	return payload.ReferenceBlockID, nil
}
