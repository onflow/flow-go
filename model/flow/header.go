// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"time"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/hash"
)

// Header contains all meta-data for a block, as well as a hash representing
// the combined payload of the entire block. It is what consensus nodes agree
// on after validating the contents against the payload hash.
type Header struct {
	Number     uint64
	Timestamp  time.Time
	Parent     crypto.Hash
	Payload    crypto.Hash
	Signatures []crypto.Signature
}

// Encode creates a serialized version of the header without the signatures, in
// order to serve as the basis for a deterministic unique block hash.
func (h Header) Encode() []byte {
	w := wrapHeader(h)
	return encoding.DefaultEncoder.MustEncode(w)
}

// Hash returns a unique hash to singularly identify the header and its block
// within the flow system.
func (h Header) Hash() crypto.Hash {
	return hash.DefaultHasher.ComputeHash(h.Encode())
}

type headerWrapper struct {
	Number    uint64
	Timestamp time.Time
	Parent    crypto.Hash
	Payload   crypto.Hash
}

func wrapHeader(h Header) headerWrapper {
	return headerWrapper{
		Number:    h.Number,
		Timestamp: h.Timestamp,
		Parent:    h.Parent,
		Payload:   h.Payload,
	}
}
