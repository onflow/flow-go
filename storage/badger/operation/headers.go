package operation

import (
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/vmihailenco/msgpack/v4"

	"github.com/onflow/flow-go/model/flow"
)

// Deprecated: to be removed in mainnet27
type oldHeader struct {
	ChainID            flow.ChainID
	ParentID           flow.Identifier
	Height             uint64
	PayloadHash        flow.Identifier
	Timestamp          time.Time
	View               uint64
	ParentView         uint64
	ParentVoterIndices []byte
	ParentVoterSigData []byte
	ProposerID         flow.Identifier
	ProposerSigData    []byte
	LastViewTC         *flow.TimeoutCertificate
}

// MarshalMsgpack makes sure the timestamp is encoded in UTC.
func (h oldHeader) MarshalMsgpack() ([]byte, error) {
	// NOTE: this is just a sanity check to make sure that we don't get
	// different encodings if someone forgets to use UTC timestamps
	if h.Timestamp.Location() != time.UTC {
		h.Timestamp = h.Timestamp.UTC()
	}
	// we use an alias to avoid endless recursion; the alias will not have the
	// marshal function and encode like a raw header
	type Encodable oldHeader
	return msgpack.Marshal(Encodable(h))
}

// UnmarshalMsgpack makes sure the timestamp is decoded in UTC.
func (h *oldHeader) UnmarshalMsgpack(data []byte) error {
	// we use an alias to avoid endless recursion; the alias will not have the
	// unmarshal function and decode like a raw header
	// NOTE: for some reason, the pointer alias works for JSON to not recurse,
	// but msgpack will still recurse; we have to do an extra struct copy here
	type Decodable oldHeader
	decodable := Decodable(*h)
	err := msgpack.Unmarshal(data, &decodable)
	*h = oldHeader(decodable)
	// NOTE: Msgpack unmarshals timestamps with the local timezone, which means
	// that a block ID would suddenly be different after encoding and decoding
	// on a machine with non-UTC local time
	if h.Timestamp.Location() != time.UTC {
		h.Timestamp = h.Timestamp.UTC()
	}
	return err
}

// convertForStorage converts the current ProposalHeader struct into the previous version
// of the Header to maintain backwards compatibility with storage.
// Deprecated: to be removed in mainnet27
func convertForStorage(p *flow.ProposalHeader) *oldHeader {
	h := p.Header
	return &oldHeader{
		ChainID:            h.ChainID,
		ParentID:           h.ParentID,
		Height:             h.Height,
		PayloadHash:        h.PayloadHash,
		Timestamp:          h.Timestamp,
		View:               h.View,
		ParentView:         h.ParentView,
		ParentVoterIndices: h.ParentVoterIndices,
		ParentVoterSigData: h.ParentVoterSigData,
		ProposerID:         h.ProposerID,
		ProposerSigData:    p.ProposerSigData,
		LastViewTC:         h.LastViewTC,
	}
}

// Deprecated: to be removed in mainnet27
func (h *oldHeader) convert() *flow.ProposalHeader {
	return &flow.ProposalHeader{
		Header: &flow.Header{
			HeaderBody: flow.HeaderBody{
				ChainID:            h.ChainID,
				ParentID:           h.ParentID,
				Height:             h.Height,
				Timestamp:          h.Timestamp,
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

// InsertHeader inserts a header by block ID.
// Error returns:
//   - storage.ErrAlreadyExists if the key already exists in the database.
//   - generic error in case of unexpected failure from the database layer or encoding failure.
func InsertHeader(blockID flow.Identifier, header *flow.ProposalHeader) func(*badger.Txn) error {
	storable := convertForStorage(header)
	return insert(makePrefix(codeHeader, blockID), storable)
}

// RetrieveHeader retrieves a header by block ID.
// Error returns:
//   - storage.ErrNotFound if the key does not exist in the database
//   - generic error in case of unexpected failure from the database layer
func RetrieveHeader(blockID flow.Identifier, header *flow.ProposalHeader) func(*badger.Txn) error {
	return func(txn *badger.Txn) error {
		var storable oldHeader
		err := retrieve(makePrefix(codeHeader, blockID), &storable)(txn)
		if err != nil {
			return err
		}
		*header = *storable.convert()
		return nil
	}
}

// IndexBlockHeight indexes the height of a block. It should only be called on
// finalized blocks.
func IndexBlockHeight(height uint64, blockID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeHeightToBlock, height), blockID)
}

// LookupBlockHeight retrieves finalized blocks by height.
func LookupBlockHeight(height uint64, blockID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeHeightToBlock, height), blockID)
}

// BlockExists checks whether the block exists in the database.
// No errors are expected during normal operation.
func BlockExists(blockID flow.Identifier, blockExists *bool) func(*badger.Txn) error {
	return exists(makePrefix(codeHeader, blockID), blockExists)
}

func InsertExecutedBlock(blockID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeExecutedBlock), blockID)
}

func UpdateExecutedBlock(blockID flow.Identifier) func(*badger.Txn) error {
	return update(makePrefix(codeExecutedBlock), blockID)
}

func RetrieveExecutedBlock(blockID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeExecutedBlock), blockID)
}

// IndexCollectionGuaranteeBlock indexes a block by a collection guarantee within that block.
func IndexCollectionGuaranteeBlock(collID flow.Identifier, blockID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeCollectionBlock, collID), blockID)
}

// LookupCollectionGuaranteeBlock looks up a block by a collection guarantee within that block.
func LookupCollectionGuaranteeBlock(collID flow.Identifier, blockID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeCollectionBlock, collID), blockID)
}

// FindHeaders iterates through all headers, calling `filter` on each, and adding
// them to the `found` slice if `filter` returned true
func FindHeaders(filter func(header *flow.Header) bool, found *[]flow.Header) func(*badger.Txn) error {
	return traverse(makePrefix(codeHeader), func() (checkFunc, createFunc, handleFunc) {
		check := func(key []byte) bool {
			return true
		}
		var val flow.Header
		create := func() interface{} {
			return &val
		}
		handle := func() error {
			if filter(&val) {
				*found = append(*found, val)
			}
			return nil
		}
		return check, create, handle
	})
}
