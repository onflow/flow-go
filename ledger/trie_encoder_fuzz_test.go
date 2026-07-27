package ledger_test

import (
	"testing"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
)

func FuzzDecodeTrieProof(f *testing.F) {
	// seed: encoded valid proof from fixture
	p, _ := testutils.TrieProofFixture()
	f.Add(ledger.EncodeTrieProof(p))

	// seed: empty
	f.Add([]byte{})
	// seed: minimal truncated input
	f.Add([]byte{0x00, 0x01})

	f.Fuzz(func(t *testing.T, data []byte) {
		proof, err := ledger.DecodeTrieProof(data)
		if err != nil {
			// malformed input must return error, not partial state
			if proof != nil {
				t.Fatal("DecodeTrieProof returned non-nil proof alongside error")
			}
			return
		}
		// valid decode: proof must be non-nil
		if proof == nil {
			t.Fatal("DecodeTrieProof returned nil proof with nil error")
		}
	})
}

func FuzzDecodeTrieBatchProof(f *testing.F) {
	// seed: encoded valid batch proof from fixture
	bp, _ := testutils.TrieBatchProofFixture()
	f.Add(ledger.EncodeTrieBatchProof(bp))

	// seed: empty
	f.Add([]byte{})
	// seed: minimal truncated input
	f.Add([]byte{0x00, 0x01})

	f.Fuzz(func(t *testing.T, data []byte) {
		batch, err := ledger.DecodeTrieBatchProof(data)
		if err != nil {
			// malformed input must return error, not partial state
			if batch != nil {
				t.Fatal("DecodeTrieBatchProof returned non-nil batch alongside error")
			}
			return
		}
		// valid decode: batch must be non-nil
		if batch == nil {
			t.Fatal("DecodeTrieBatchProof returned nil batch with nil error")
		}
	})
}
