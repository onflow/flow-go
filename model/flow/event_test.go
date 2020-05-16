package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/model/encoding/rlp"
	"github.com/dapperlabs/flow-go/model/fingerprint"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type eventWrapper struct {
	TxID  []byte
	Index uint32
}

func wrapEvent(e flow.Event) eventWrapper {
	return eventWrapper{
		TxID:  e.TransactionID[:],
		Index: e.EventIndex,
	}
}

func TestEventFingerprint(t *testing.T) {
	evt := unittest.EventFixture(flow.EventAccountCreated, 13, 12, unittest.IdentifierFixture())
	data := fingerprint.Fingerprint(evt)
	var decoded eventWrapper
	rlp.NewEncoder().MustDecode(data, &decoded)
	assert.Equal(t, wrapEvent(evt), decoded)
}
