package flow

import (
	"encoding/json"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/vmihailenco/msgpack/v4"

	cborcodec "github.com/onflow/flow-go/model/encoding/cbor"
	"github.com/onflow/flow-go/model/fingerprint"
)

// ProposalHeader is a block header and the proposer's signature for the block.
type ProposalHeader struct {
	Header *Header
	// ProposerSigData is a signature of the proposer over the new block. Not a single cryptographic
	// signature since the data represents cryptographic signatures serialized in some way (concatenation or other)
	ProposerSigData []byte
}

// HeaderBody contains all block header metadata, except for the payload hash.
// HeaderBody generally should not be used on its own. It is merely a container used by other
// data structures in the code base. For example, it is embedded within [Block], [Header], and the
// respective collector cluster structs - those types should be used in almost all circumstances.
// CAUTION regarding security:
//   - HeaderBody does not contain the hash of the block payload. Therefore, it is not a cryptographic digest
//     of the block and should not be confused with a "proper" header, which commits to the _entire_ content
//     of a block.
//   - With a byzantine HeaderBody alone, an honest node cannot prove who created that faulty data structure,
//     because HeaderBody does not include the proposer's signature.
type HeaderBody struct {
	// ChainID is a chain-specific value to prevent replay attacks.
	ChainID ChainID
	// ParentID is the ID of this block's parent.
	ParentID Identifier
	// Height is the height of the parent + 1
	Height uint64
	// Timestamp is the time at which this block was proposed.
	// The proposer can choose any time, so this should not be trusted as accurate.
	Timestamp time.Time
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

// QuorumCertificate returns quorum certificate [QC] that is incorporated in the block header body.
// Caution: this is the QC for the parent.
func (h HeaderBody) QuorumCertificate() *QuorumCertificate {
	return &QuorumCertificate{
		BlockID:       h.ParentID,
		View:          h.ParentView,
		SignerIndices: h.ParentVoterIndices,
		SigData:       h.ParentVoterSigData,
	}
}

// Header contains all meta-data for a block, as well as a hash of the block payload.
// Headers are used when the metadata about a block is needed, but the payload is not.
// Because [Header] includes the payload hash for the block, and the block ID is Merkle-ized
// with the Payload field as a Merkle tree node, the block ID can be computed from the [Header].
// CAUTION regarding security:
//   - With a byzantine HeaderBody alone, an honest node cannot prove who created that faulty data structure,
//     because HeaderBody does not include the proposer's signature.
type Header struct {
	HeaderBody
	// PayloadHash is a hash of the payload of this block.
	PayloadHash Identifier
}

// Fingerprint defines custom encoding for the header to calculate its ID.
// Timestamp is converted from time.Time to unix time (uint64), which is necessary
// because time.Time is not RLP-encodable (due to having private fields).
func (h Header) Fingerprint() []byte {
	return fingerprint.Fingerprint(struct {
		ChainID            ChainID
		ParentID           Identifier
		Height             uint64
		PayloadHash        Identifier
		Timestamp          uint64
		View               uint64
		ParentView         uint64
		ParentVoterIndices []byte
		ParentVoterSigData []byte
		ProposerID         Identifier
		LastViewTCID       Identifier
	}{
		ChainID:            h.ChainID,
		ParentID:           h.ParentID,
		Height:             h.Height,
		PayloadHash:        h.PayloadHash,
		Timestamp:          uint64(h.Timestamp.UnixNano()),
		View:               h.View,
		ParentView:         h.ParentView,
		ParentVoterIndices: h.ParentVoterIndices,
		ParentVoterSigData: h.ParentVoterSigData,
		ProposerID:         h.ProposerID,
		LastViewTCID:       h.LastViewTC.ID(),
	})
}

// ID returns a unique ID to singularly identify the header and its block
// within the flow system.
func (h Header) ID() Identifier {
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

// OldHeader is only used to temporarily provide backwards compatibility for network messages and storage.
// Deprecated: to be removed in mainnet27.
type OldHeader struct {
	ChainID            ChainID
	ParentID           Identifier
	Height             uint64
	PayloadHash        Identifier
	Timestamp          time.Time
	View               uint64
	ParentView         uint64
	ParentVoterIndices []byte
	ParentVoterSigData []byte
	ProposerID         Identifier
	ProposerSigData    []byte
	LastViewTC         *TimeoutCertificate
}

// ConvertToOldHeader converts the current ProposalHeader struct into the previous version
// of the Header to maintain backwards compatibility with storage and network messages.
// Deprecated: to be removed in mainnet27
func ConvertToOldHeader(h *Header, proposerSig []byte) *OldHeader {
	timestamp := h.Timestamp
	if timestamp.Location() != time.UTC {
		timestamp = timestamp.UTC()
	}
	return &OldHeader{
		ChainID:            h.ChainID,
		ParentID:           h.ParentID,
		Height:             h.Height,
		PayloadHash:        h.PayloadHash,
		Timestamp:          timestamp,
		View:               h.View,
		ParentView:         h.ParentView,
		ParentVoterIndices: h.ParentVoterIndices,
		ParentVoterSigData: h.ParentVoterSigData,
		ProposerID:         h.ProposerID,
		ProposerSigData:    proposerSig,
		LastViewTC:         h.LastViewTC,
	}
}

// ConvertToNewHeader converts the old version of the Header (still used in network messages and storage
// for backwards compatibility) to the new version already being used internally.
// Deprecated: to be removed in mainnet27
func (h *OldHeader) ConvertToNewHeader() *ProposalHeader {
	timestamp := h.Timestamp
	if timestamp.Location() != time.UTC {
		timestamp = timestamp.UTC()
	}
	return &ProposalHeader{
		Header: &Header{
			HeaderBody: HeaderBody{
				ChainID:            h.ChainID,
				ParentID:           h.ParentID,
				Height:             h.Height,
				Timestamp:          timestamp,
				View:               h.View,
				ParentView:         h.ParentView,
				ParentVoterIndices: h.ParentVoterIndices,
				ParentVoterSigData: h.ParentVoterSigData,
				ProposerID:         h.ProposerID,
				LastViewTC:         h.LastViewTC,
			},
			PayloadHash: h.PayloadHash,
		},
		ProposerSigData: h.ProposerSigData,
	}
}

// MarshalJSON adds the block ID to the output.
// [OldHeader.Timestamp] UTC timezone is already set in [ConvertToOldHeader].
func (h OldHeader) MarshalJSON() ([]byte, error) {
	// we use an alias to avoid endless recursion; the alias will not have the
	// marshal function and encode like a raw header
	type Encodable OldHeader
	return json.Marshal(struct {
		Encodable
		ID string
	}{
		Encodable: Encodable(h),
		ID:        h.ConvertToNewHeader().Header.ID().String(),
	})
}
