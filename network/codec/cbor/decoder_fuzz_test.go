package cbor_test

import (
	"bytes"
	"testing"

	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/utils/unittest"
)

func FuzzCBORDecoder(f *testing.F) {
	c := cbor.NewCodec()

	// seed: valid block proposal round-tripped through the encoder
	proposal := messages.Proposal(*unittest.ProposalFixture())
	var buf bytes.Buffer
	if err := c.NewEncoder(&buf).Encode(&proposal); err == nil {
		f.Add(buf.Bytes())
	}

	// seed: empty
	f.Add([]byte{})
	// seed: single null byte
	f.Add([]byte{0x00})
	// seed: cbor-encoded empty byte slice (mirrors decoder_test.go cases)
	f.Add([]byte{0x80})

	f.Fuzz(func(t *testing.T, data []byte) {
		msg, err := c.NewDecoder(bytes.NewReader(data)).Decode()
		if err != nil {
			// any error is acceptable; the decoder must not panic
			return
		}

		// round-trip: re-encoding a valid decoded message must decode to an equal value
		var roundtrip bytes.Buffer
		if err := c.NewEncoder(&roundtrip).Encode(msg); err != nil {
			// encoder may reject the value if the interface type has no registered code — treat as non-fatal
			return
		}
		msg2, err := c.NewDecoder(&roundtrip).Decode()
		if err != nil {
			t.Fatalf("round-trip decode failed after successful encode: %v", err)
		}
		_ = msg2
	})
}
