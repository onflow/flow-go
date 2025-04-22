package flow

import (
	"encoding/json"

	"github.com/fxamacker/cbor/v2"
	"github.com/vmihailenco/msgpack/v4"

	cborcodec "github.com/onflow/flow-go/model/encoding/cbor"
)

// Proposal is a block header and the proposer's signature for the block.
type Proposal struct {
	Header *Header
	// ProposerSigData is a signature of the proposer over the new block. Not a single cryptographic
	// signature since the data represents cryptographic signatures serialized in some way (concatenation or other)
	ProposerSigData []byte
}

// Header contains all meta-data for a block, as well as a hash representing
// the combined payload of the entire block. It is what consensus nodes agree
// on after validating the contents against the payload hash.
type Header struct {
	// ChainID is a chain-specific value to prevent replay attacks.
	ChainID ChainID
	// ParentID is the ID of this block's parent.
	ParentID Identifier
	// Height is the height of the parent + 1
	Height uint64
	// PayloadHash is a hash of the payload of this block.
	PayloadHash Identifier
	// Timestamp is the time at which this block was proposed, in Unix milliseconds.
	// The proposer can choose any time, so this should not be trusted as accurate.
	Timestamp uint64
	// View number at which this block was proposed.
	View uint64
	// ParentView number at which parent block was proposed.
	ParentView uint64
	// ParentVoterIndices is a bitvector that represents all the voters for the parent block.
	ParentVoterIndices []byte
	// ParentVoterSigData is an aggregated signature over the parent block. Not a single cryptographic
	// signature since the data represents cryptographic signatures serialized in some way (concatenation or other)
	// A quorum certificate can be extracted from the header.
	// This field is the SigData field of the extracted quorum certificate.
	ParentVoterSigData []byte
	// ProposerID is a proposer identifier for the block
	ProposerID Identifier
	// LastViewTC is a timeout certificate for previous view, it can be nil
	// it has to be present if previous round ended with timeout.
	LastViewTC *TimeoutCertificate
}

// QuorumCertificate returns quorum certificate that is incorporated in the block header.
func (h Header) QuorumCertificate() *QuorumCertificate {
	return &QuorumCertificate{
		BlockID:       h.ParentID,
		View:          h.ParentView,
		SignerIndices: h.ParentVoterIndices,
		SigData:       h.ParentVoterSigData,
	}
}

// ID returns a unique ID to singularly identify the header and its block
// within the flow system.
func (h Header) ID() Identifier {
	return MakeID(h)
}

// Checksum returns the checksum of the header.
func (h Header) Checksum() Identifier {
	return MakeID(h)
}

// MarshalJSON makes sure the timestamp is encoded in UTC.
func (h Header) MarshalJSON() ([]byte, error) {

	// we use an alias to avoid endless recursion; the alias will not have the
	// marshal function and encode like a raw header
	type Encodable Header
	return json.Marshal(struct {
		Encodable
		ID string
	}{
		Encodable: Encodable(h),
		ID:        h.ID().String(),
	})
}

// UnmarshalJSON makes sure the timestamp is decoded in UTC.
func (h *Header) UnmarshalJSON(data []byte) error {

	// we use an alias to avoid endless recursion; the alias will not have the
	// unmarshal function and decode like a raw header
	type Decodable *Header
	err := json.Unmarshal(data, Decodable(h))

	return err
}

// MarshalCBOR makes sure the timestamp is encoded in UTC.
func (h Header) MarshalCBOR() ([]byte, error) {

	// we use an alias to avoid endless recursion; the alias will not have the
	// marshal function and encode like a raw header
	type Encodable Header
	return cborcodec.EncMode.Marshal(Encodable(h))
}

// UnmarshalCBOR makes sure the timestamp is decoded in UTC.
func (h *Header) UnmarshalCBOR(data []byte) error {

	// we use an alias to avoid endless recursion; the alias will not have the
	// unmarshal function and decode like a raw header
	// NOTE: for some reason, the pointer alias works for JSON to not recurse,
	// but msgpack will still recurse; we have to do an extra struct copy here
	type Decodable Header
	decodable := Decodable(*h)
	err := cbor.Unmarshal(data, &decodable)
	*h = Header(decodable)

	return err
}

// MarshalMsgpack makes sure the timestamp is encoded in UTC.
func (h Header) MarshalMsgpack() ([]byte, error) {

	// we use an alias to avoid endless recursion; the alias will not have the
	// marshal function and encode like a raw header
	type Encodable Header
	return msgpack.Marshal(Encodable(h))
}

// UnmarshalMsgpack makes sure the timestamp is decoded in UTC.
func (h *Header) UnmarshalMsgpack(data []byte) error {

	// we use an alias to avoid endless recursion; the alias will not have the
	// unmarshal function and decode like a raw header
	// NOTE: for some reason, the pointer alias works for JSON to not recurse,
	// but msgpack will still recurse; we have to do an extra struct copy here
	type Decodable Header
	decodable := Decodable(*h)
	err := msgpack.Unmarshal(data, &decodable)
	*h = Header(decodable)

	return err
}
