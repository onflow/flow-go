package flow

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/vmihailenco/msgpack/v4"

	"github.com/onflow/flow-go/ledger/common/bitutils"
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
//
//structwrite:immutable - mutations allowed only within the constructor
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

// UntrustedHeaderBody is an untrusted input-only representation of a HeaderBody,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedHeaderBody should be validated and converted into
// a trusted HeaderBody using NewHeaderBody constructor.
type UntrustedHeaderBody HeaderBody

// NewHeaderBody creates a new instance of HeaderBody.
// Construction of HeaderBody is allowed only within the constructor
//
// All errors indicate a valid HeaderBody cannot be constructed from the input.
func NewHeaderBody(untrusted UntrustedHeaderBody) (*HeaderBody, error) {
	if untrusted.ChainID == "" {
		return nil, fmt.Errorf("ChainID must not be empty")
	}
	if untrusted.ParentID == ZeroID {
		return nil, fmt.Errorf("ParentID must not be empty")
	}
	if untrusted.Timestamp.IsZero() {
		return nil, fmt.Errorf("Timestamp must not be zero‐value")
	}
	if len(untrusted.ParentVoterIndices) == 0 {
		return nil, fmt.Errorf("ParentVoterIndices must not be empty")
	}
	if len(untrusted.ParentVoterSigData) == 0 {
		return nil, fmt.Errorf("ParentVoterSigData must not be empty")
	}
	if untrusted.ProposerID == ZeroID {
		return nil, fmt.Errorf("ProposerID must not be empty")
	}

	return &HeaderBody{
		ChainID:            untrusted.ChainID,
		ParentID:           untrusted.ParentID,
		Height:             untrusted.Height,
		Timestamp:          untrusted.Timestamp,
		View:               untrusted.View,
		ParentView:         untrusted.ParentView,
		ParentVoterIndices: untrusted.ParentVoterIndices,
		ParentVoterSigData: untrusted.ParentVoterSigData,
		ProposerID:         untrusted.ProposerID,
		LastViewTC:         untrusted.LastViewTC,
	}, nil
}

// NewRootHeaderBody creates a new instance of root HeaderBody.
// This constructor must be used **only** for constructing the root header body,
// which is the only case where zero values are allowed.
func NewRootHeaderBody(untrusted UntrustedHeaderBody) (*HeaderBody, error) {
	if untrusted.ChainID == "" {
		return nil, fmt.Errorf("ChainID of root header body must not be empty")
	}

	if untrusted.ParentView != 0 {
		return nil, fmt.Errorf("ParentView of root header body must be zero")
	}

	if len(untrusted.ParentVoterIndices) != 0 {
		return nil, fmt.Errorf("ParentVoterIndices of root header body must be empty")
	}
	if len(untrusted.ParentVoterSigData) != 0 {
		return nil, fmt.Errorf("ParentVoterSigData of root header body must be empty")
	}

	return &HeaderBody{
		ChainID:            untrusted.ChainID,
		ParentID:           untrusted.ParentID,
		Height:             untrusted.Height,
		Timestamp:          untrusted.Timestamp,
		View:               untrusted.View,
		ParentView:         untrusted.ParentView,
		ParentVoterIndices: untrusted.ParentVoterIndices,
		ParentVoterSigData: untrusted.ParentVoterSigData,
		ProposerID:         untrusted.ProposerID,
		LastViewTC:         untrusted.LastViewTC,
	}, nil
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

// ContainsParentQC reports whether this header carries a valid parent QC.
// It returns true only if all of the fields required to build a QC are non-zero/nil,
// indicating that ParentQC() can be safely called without panicking.
// Only spork root blocks or network genesis blocks do not contain a parent QC.
func (h HeaderBody) ContainsParentQC() bool {
	return h.ParentID != ZeroID &&
		h.ParentVoterIndices != nil &&
		h.ParentVoterSigData != nil &&
		h.ProposerID != ZeroID
}

// Header contains all meta-data for a block, as well as a hash of the block payload.
// Headers are used when the metadata about a block is needed, but the payload is not.
// Because [Header] includes the payload hash for the block, and the block ID is Merkle-ized
// with the Payload field as a Merkle tree node, the block ID can be computed from the [Header].
// CAUTION regarding security:
//   - With a byzantine HeaderBody alone, an honest node cannot prove who created that faulty data structure,
//     because HeaderBody does not include the proposer's signature.
//
//structwrite:immutable - mutations allowed only within the constructor
type Header struct {
	HeaderBody
	// PayloadHash is a hash of the payload of this block.
	PayloadHash Identifier
}

// UntrustedHeader is an untrusted input-only representation of a Header,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedHeader should be validated and converted into
// a trusted Header using NewHeader constructor.
type UntrustedHeader Header

// NewHeader creates a new instance of Header.
// Construction of Header is allowed only within the constructor
//
// All errors indicate a valid Header cannot be constructed from the input.
func NewHeader(untrusted UntrustedHeader) (*Header, error) {
	headerBody, err := NewHeaderBody(UntrustedHeaderBody(untrusted.HeaderBody))
	if err != nil {
		return nil, fmt.Errorf("invalid header body: %w", err)
	}

	if untrusted.PayloadHash == ZeroID {
		return nil, fmt.Errorf("PayloadHash must not be empty")
	}

	return &Header{
		HeaderBody:  *headerBody,
		PayloadHash: untrusted.PayloadHash,
	}, nil
}

// NewRootHeader creates a root header.
//
// This constructor must be used **only** for constructing the root header,
// which is the only case where zero values are allowed.
func NewRootHeader(untrusted UntrustedHeader) (*Header, error) {
	rootHeaderBody, err := NewRootHeaderBody(UntrustedHeaderBody(untrusted.HeaderBody))
	if err != nil {
		return nil, fmt.Errorf("invalid root header body: %w", err)
	}

	return &Header{
		HeaderBody:  *rootHeaderBody,
		PayloadHash: untrusted.PayloadHash,
	}, nil
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

type HeaderBodyFields int

const (
	ChainIdentifier HeaderBodyFields = iota
	ParentIdentifier
	Height
	Timestamp
	View
	ParentView
	ParentVoterIndices
	ParentVoterSigData
	ProposerIdentifier
	numHeaderBodyFields // always keep this last
)

// HeaderBodyBuilder constructs a validated, immutable HeaderBody in two phases:
// first by setting individual fields using fluent WithX methods, then by calling Build()
// to perform centralized validation and return the final HeaderBody.
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

// Build validates and returns an immutable Header. All required fields must be explicitly set (even if they are zero).
// All errors indicate that a valid HeaderBody cannot be created from the current builder state.
func (b *HeaderBodyBuilder) Build() (*HeaderBody, error) {
	// make sure every required field was initialized
	for bit := 0; bit < int(numHeaderBodyFields); bit++ {
		if bitutils.ReadBit(b.present, bit) == 0 {
			return nil, fmt.Errorf("HeaderBodyBuilder: missing field at bit index %d", bit)
		}
	}

	return NewHeaderBody(b.u)
}

func (h *HeaderBodyBuilder) WithChainID(id ChainID) *HeaderBodyBuilder {
	h.u.ChainID = id
	bitutils.WriteBit(h.present, int(ChainIdentifier), 1)
	return h
}
func (h *HeaderBodyBuilder) WithParentID(pid Identifier) *HeaderBodyBuilder {
	h.u.ParentID = pid
	bitutils.WriteBit(h.present, int(ParentIdentifier), 1)
	return h
}
func (h *HeaderBodyBuilder) WithHeight(height uint64) *HeaderBodyBuilder {
	h.u.Height = height
	bitutils.WriteBit(h.present, int(Height), 1)
	return h
}
func (h *HeaderBodyBuilder) WithTimestamp(t time.Time) *HeaderBodyBuilder {
	h.u.Timestamp = t
	bitutils.WriteBit(h.present, int(Timestamp), 1)
	return h
}
func (h *HeaderBodyBuilder) WithView(v uint64) *HeaderBodyBuilder {
	h.u.View = v
	bitutils.WriteBit(h.present, int(View), 1)
	return h
}
func (h *HeaderBodyBuilder) WithParentView(pv uint64) *HeaderBodyBuilder {
	h.u.ParentView = pv
	bitutils.WriteBit(h.present, int(ParentView), 1)
	return h
}
func (h *HeaderBodyBuilder) WithParentVoterIndices(idx []byte) *HeaderBodyBuilder {
	h.u.ParentVoterIndices = idx
	bitutils.WriteBit(h.present, int(ParentVoterIndices), 1)
	return h
}
func (h *HeaderBodyBuilder) WithParentVoterSigData(sig []byte) *HeaderBodyBuilder {
	h.u.ParentVoterSigData = sig
	bitutils.WriteBit(h.present, int(ParentVoterSigData), 1)
	return h
}
func (h *HeaderBodyBuilder) WithProposerID(id Identifier) *HeaderBodyBuilder {
	h.u.ProposerID = id
	bitutils.WriteBit(h.present, int(ProposerIdentifier), 1)
	return h
}
func (h *HeaderBodyBuilder) WithLastViewTC(tc *TimeoutCertificate) *HeaderBodyBuilder {
	h.u.LastViewTC = tc
	return h
}

// MarshalJSON makes sure the timestamp is encoded in UTC.
//
// TODO(malleability): remove when PR #7325 will be merged (convert Header.Timestamp to Unix Milliseconds)
//
//nolint:structwrite
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
//
// TODO(malleability): remove when PR #7325 will be merged (convert Header.Timestamp to Unix Milliseconds)
//
//nolint:structwrite
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
//
// TODO(malleability): remove when PR #7325 will be merged (convert Header.Timestamp to Unix Milliseconds)
//
//nolint:structwrite
func (h Header) MarshalCBOR() ([]byte, error) {

	// NOTE: this is just a sanity check to make sure that we don't get
	// different encodings if someone forgets to use UTC timestamps
	if h.Timestamp.Location() != time.UTC {
		h.Timestamp = h.Timestamp.UTC() //nolint:structwrite
	}

	// we use an alias to avoid endless recursion; the alias will not have the
	// marshal function and encode like a raw header
	type Encodable Header
	return cborcodec.EncMode.Marshal(Encodable(h))
}

// UnmarshalCBOR makes sure the timestamp is decoded in UTC.
//
// TODO(malleability): remove when PR #7325 will be merged (convert Header.Timestamp to Unix Milliseconds)
//
//nolint:structwrite
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
//
// TODO(malleability): remove when PR #7325 will be merged (convert Header.Timestamp to Unix Milliseconds)
//
//nolint:structwrite
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
//
// TODO(malleability): remove when PR #7325 will be merged (convert Header.Timestamp to Unix Milliseconds)
//
//nolint:structwrite
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
