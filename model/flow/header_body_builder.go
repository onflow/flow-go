package flow

import (
	"fmt"

	"github.com/onflow/flow-go/ledger/common/bitutils"
)

// headerBodyFieldBitIndex enumerates required fields in HeaderBody so that HeaderBodyBuilder
// can enforce that all required fields are explicitly set (including to zero values) prior to building.
type headerBodyFieldBitIndex int

const (
	chainIDFieldBitIndex headerBodyFieldBitIndex = iota
	parentIDFieldBitIndex
	heightFieldBitIndex
	timestampFieldBitIndex
	viewFieldBitIndex
	parentViewFieldBitIndex
	parentVoterIndicesFieldBitIndex
	parentVoterSigDataFieldBitIndex
	proposerIDFieldBitIndex
	numHeaderBodyFields // always keep this last
)

// String returns the name of the field corresponding to this bit index.
func (f headerBodyFieldBitIndex) String() string {
	switch f {
	case chainIDFieldBitIndex:
		return "ChainID"
	case parentIDFieldBitIndex:
		return "ParentID"
	case heightFieldBitIndex:
		return "Height"
	case timestampFieldBitIndex:
		return "Timestamp"
	case viewFieldBitIndex:
		return "View"
	case parentViewFieldBitIndex:
		return "ParentView"
	case parentVoterIndicesFieldBitIndex:
		return "ParentVoterIndices"
	case parentVoterSigDataFieldBitIndex:
		return "ParentVoterSigData"
	case proposerIDFieldBitIndex:
		return "ProposerID"
	default:
		return fmt.Sprintf("UnknownField(%d)", int(f))
	}
}

// HeaderBodyBuilder constructs a validated, immutable HeaderBody in two phases:
// first by setting individual fields using fluent WithX methods, then by calling Build()
// to perform minimal validity and sanity checks and return the final [HeaderBody].
type HeaderBodyBuilder struct {
	u       UntrustedHeaderBody
	present []byte
}

// NewHeaderBodyBuilder helps to build a new Header.
func NewHeaderBodyBuilder() *HeaderBodyBuilder {
	return &HeaderBodyBuilder{
		present: bitutils.MakeBitVector(int(numHeaderBodyFields)),
	}
}

// Build validates and returns an immutable HeaderBody. All required fields must be explicitly set (even if they are zero).
// All errors indicate that a valid HeaderBody cannot be created from the current builder state.
func (b *HeaderBodyBuilder) Build() (*HeaderBody, error) {
	// make sure every required field was initialized
	for bit := range int(numHeaderBodyFields) {
		if bitutils.ReadBit(b.present, bit) == 0 {
			return nil, fmt.Errorf("HeaderBodyBuilder: missing field %s", headerBodyFieldBitIndex(bit))
		}
	}

	return NewHeaderBody(b.u)
}

func (h *HeaderBodyBuilder) WithChainID(id ChainID) *HeaderBodyBuilder {
	h.u.ChainID = id
	bitutils.WriteBit(h.present, int(chainIDFieldBitIndex), 1)
	return h
}
func (h *HeaderBodyBuilder) WithParentID(pid Identifier) *HeaderBodyBuilder {
	h.u.ParentID = pid
	bitutils.WriteBit(h.present, int(parentIDFieldBitIndex), 1)
	return h
}
func (h *HeaderBodyBuilder) WithHeight(height uint64) *HeaderBodyBuilder {
	h.u.Height = height
	bitutils.WriteBit(h.present, int(heightFieldBitIndex), 1)
	return h
}
func (h *HeaderBodyBuilder) WithTimestamp(t uint64) *HeaderBodyBuilder {
	h.u.Timestamp = t
	bitutils.WriteBit(h.present, int(timestampFieldBitIndex), 1)
	return h
}
func (h *HeaderBodyBuilder) WithView(v uint64) *HeaderBodyBuilder {
	h.u.View = v
	bitutils.WriteBit(h.present, int(viewFieldBitIndex), 1)
	return h
}
func (h *HeaderBodyBuilder) WithParentView(pv uint64) *HeaderBodyBuilder {
	h.u.ParentView = pv
	bitutils.WriteBit(h.present, int(parentViewFieldBitIndex), 1)
	return h
}
func (h *HeaderBodyBuilder) WithParentVoterIndices(idx []byte) *HeaderBodyBuilder {
	h.u.ParentVoterIndices = idx
	bitutils.WriteBit(h.present, int(parentVoterIndicesFieldBitIndex), 1)
	return h
}
func (h *HeaderBodyBuilder) WithParentVoterSigData(sig []byte) *HeaderBodyBuilder {
	h.u.ParentVoterSigData = sig
	bitutils.WriteBit(h.present, int(parentVoterSigDataFieldBitIndex), 1)
	return h
}
func (h *HeaderBodyBuilder) WithProposerID(id Identifier) *HeaderBodyBuilder {
	h.u.ProposerID = id
	bitutils.WriteBit(h.present, int(proposerIDFieldBitIndex), 1)
	return h
}
func (h *HeaderBodyBuilder) WithLastViewTC(tc *TimeoutCertificate) *HeaderBodyBuilder {
	h.u.LastViewTC = tc
	return h
}
