package badger

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Translator is a translation layer that determines the reference block on
// the main chain for a given cluster block. For now, just returns the genesis
// block.
//
//TODO: Currently this always returns genesis ID, which will work until epochs
// and in-epoch slashing are implemented. Need to update this to translate
// based on the ReferenceBlockID specified in the cluster block.
type Translator struct {
	// cache the genesis block ID and always return it for now
	genesisID flow.Identifier
}

// NewTranslator returns a new block ID translator.
func NewTranslator(genesisID flow.Identifier) *Translator {
	translator := &Translator{
		genesisID: genesisID,
	}
	return translator
}

// Translate retrieves the reference main-chain block ID for the given cluster
// block ID.
// TODO: update to use ReferenceBlockID
func (t *Translator) Translate(blockID flow.Identifier) (flow.Identifier, error) {
	return t.genesisID, nil
}
