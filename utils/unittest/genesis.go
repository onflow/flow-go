package unittest

import (
	"time"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Genesis returns a genesis block containing the given node identities.
func Genesis(identities flow.IdentityList) *flow.Block {
	header := flow.Header{
		Number:    0,
		Timestamp: time.Now().UTC(),
		Parent:    crypto.ZeroHash,
	}

	genesis := flow.Block{
		Header:        header,
		NewIdentities: identities,
	}

	genesis.Header.Payload = genesis.Payload()

	return &genesis
}
