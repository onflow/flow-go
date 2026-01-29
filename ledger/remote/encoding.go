package remote

import (
	"github.com/onflow/flow-go/ledger"
)

// encodeTrieUpdateForTransport encodes a trie update for transmission over gRPC.
// This function MUST be used by the server when encoding trie updates.
// The client MUST use decodeTrieUpdateFromTransport to decode.
//
// This centralized function ensures that both client and server use the same
// encoding method, preventing encoding/decoding mismatches.
//
// Currently uses CBOR encoding to preserve the distinction between nil and []byte{}
// values in payloads. If the encoding method needs to change, update this function
// and ensure decodeTrieUpdateFromTransport uses the matching decoder.
func encodeTrieUpdateForTransport(trieUpdate *ledger.TrieUpdate) []byte {
	return ledger.EncodeTrieUpdateCBOR(trieUpdate)
}

// decodeTrieUpdateFromTransport decodes a trie update received over gRPC.
// This function MUST be used by the client when decoding trie updates.
// The server MUST use encodeTrieUpdateForTransport to encode.
//
// This centralized function ensures that both client and server use the same
// encoding method, preventing encoding/decoding mismatches.
//
// Currently uses CBOR decoding to preserve the distinction between nil and []byte{}
// values in payloads. If the encoding method needs to change, update this function
// and ensure encodeTrieUpdateForTransport uses the matching encoder.
func decodeTrieUpdateFromTransport(encodedTrieUpdate []byte) (*ledger.TrieUpdate, error) {
	return ledger.DecodeTrieUpdateCBOR(encodedTrieUpdate)
}
