package mock

import (
	"time"

	"github.com/dapperlabs/flow-go/model/flow"
)

// Genesis returns a genesis block containing the given node identities.
func Genesis(identities flow.IdentityList) *flow.Block {

	content := flow.Content{
		Identities: identities,
		Guarantees: nil,
	}

	payload := content.Payload()

	header := flow.Header{
		Number:      0,
		Timestamp:   time.Now().UTC(),
		ParentID:    flow.ZeroID,
		PayloadHash: payload.Root(),
	}

	genesis := flow.Block{
		Header:  header,
		Payload: payload,
		Content: content,
	}

	return &genesis
}
