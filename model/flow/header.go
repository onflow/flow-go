package flow

import (
	"encoding/json"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/vmihailenco/msgpack/v4"

	"github.com/onflow/flow-go/crypto"
	cborcodec "github.com/onflow/flow-go/model/encoding/cbor"
	"github.com/onflow/flow-go/model/fingerprint"
)

// Header contains all meta-data for a block, as well as a hash representing
// the combined payload of the entire block. It is what consensus nodes agree
// on after validating the contents against the payload hash.
type Header struct {
	ChainID ChainID // ChainID is a chain-specific value to prevent replay attacks.

	ParentID Identifier // ParentID is the ID of this block's parent.

	Height uint64 // Height is the height of the parent + 1

	PayloadHash Identifier // PayloadHash is a hash of the payload of this block.

	Timestamp time.Time // Timestamp is the time at which this block was proposed.

	View uint64 // View number at which this block was proposed.

	ParentVoterIDs []Identifier // List of voters who signed the parent block. Used as QC.SignerIDs

	ParentVoterSig []byte // Used as QC.SigData. Might be a serialization of multiple signatures and therefore not a crypto.Signature.

	ProposerID Identifier // proposer identifier for the block

	ProposerSig []byte // signature of the proposer over the new block.
	// Might be a serialization of multiple signatures and therefore not a crypto.Signature.
}

// Body returns the immutable part of the block header.
func (h Header) Body() interface{} {
	return struct {
		ChainID        ChainID
		ParentID       Identifier
		Height         uint64
		PayloadHash    Identifier
		Timestamp      uint64
		View           uint64
		ParentVoterIDs []Identifier
		ParentVoterSig crypto.Signature
		ProposerID     Identifier
	}{
		ChainID:        h.ChainID,
		ParentID:       h.ParentID,
		Height:         h.Height,
		PayloadHash:    h.PayloadHash,
		Timestamp:      uint64(h.Timestamp.UnixNano()),
		View:           h.View,
		ParentVoterIDs: h.ParentVoterIDs,
		ParentVoterSig: h.ParentVoterSig,
		ProposerID:     h.ProposerID,
	}
}

func (h Header) Fingerprint() []byte {
	return fingerprint.Fingerprint(h.Body())
}

// ID returns a unique ID to singularly identify the header and its block
// within the flow system.
func (h Header) ID() Identifier {
	// NOTE: this is just a sanity check to make sure that we don't get
	// different block IDs if someone forgets to use UTC timestamps
	if h.Timestamp.Location() != time.UTC {
		h.Timestamp = h.Timestamp.UTC()
	}
	return MakeID(h)
}

// Checksum returns the checksum of the header.
func (h Header) Checksum() Identifier {
	return MakeID(h)
}

// MarshalJSON makes sure the timestamp is encoded in UTC.
func (h Header) MarshalJSON() ([]byte, error) {

	// NOTE: this is just a sanity check to make sure that we don't get
	// different encodings if someone forgets to use UTC timestamps
	if h.Timestamp.Location() != time.UTC {
		h.Timestamp = h.Timestamp.UTC()
	}

	// we use an alias to avoid endless recursion; the alias will not have the
	// marshal function and encode like a raw header
	type Encodable Header
	return json.Marshal(Encodable(h))
}

// UnmarshalJSON makes sure the timestamp is decoded in UTC.
func (h *Header) UnmarshalJSON(data []byte) error {

	// we use an alias to avoid endless recursion; the alias will not have the
	// unmarshal function and decode like a raw header
	type Decodable *Header
	err := json.Unmarshal(data, Decodable(h))

	// NOTE: the timezone check is not required for JSON, as it already encodes
	// timezones, but it doesn't hurt to add it in case someone messes with the
	// raw encoded format
	if h.Timestamp.Location() != time.UTC {
		h.Timestamp = h.Timestamp.UTC()
	}

	return err
}

// MarshalCBOR makes sure the timestamp is encoded in UTC.
func (h Header) MarshalCBOR() ([]byte, error) {

	// NOTE: this is just a sanity check to make sure that we don't get
	// different encodings if someone forgets to use UTC timestamps
	if h.Timestamp.Location() != time.UTC {
		h.Timestamp = h.Timestamp.UTC()
	}

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

	// NOTE: the timezone check is not required for CBOR, as it already encodes
	// timezones, but it doesn't hurt to add it in case someone messes with the
	// raw encoded format
	if h.Timestamp.Location() != time.UTC {
		h.Timestamp = h.Timestamp.UTC()
	}

	return err
}

// MarshalMsgpack makes sure the timestamp is encoded in UTC.
func (h Header) MarshalMsgpack() ([]byte, error) {

	// NOTE: this is just a sanity check to make sure that we don't get
	// different encodings if someone forgets to use UTC timestamps
	if h.Timestamp.Location() != time.UTC {
		h.Timestamp = h.Timestamp.UTC()
	}

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

	// NOTE: Msgpack unmarshals timestamps with the local timezone, which means
	// that a block ID would suddenly be different after encoding and decoding
	// on a machine with non-UTC local time
	if h.Timestamp.Location() != time.UTC {
		h.Timestamp = h.Timestamp.UTC()
	}

	return err
}
