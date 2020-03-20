// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"time"

	"github.com/dapperlabs/flow-go/crypto"
)

func Genesis(identities IdentityList) *Block {

	// create the first seal with zero references
	seal := Seal{
		BlockID:       ZeroID,
		PreviousState: nil,
		FinalState:    GenesisStateCommitment,
	}

	// create the raw content for the genesis block
	payload := Payload{
		Identities: identities,
		Guarantees: nil,
		Seals:      []*Seal{&seal},
	}

	// create the header
	header := Header{
		ChainID:     DefaultChainID,
		ParentID:    ZeroID,
		Height:      0,
		PayloadHash: payload.Hash(),
		Timestamp:   GenesisTime(),
	}

	// combine to block
	genesis := Block{
		Header:  header,
		Payload: payload,
	}

	return &genesis
}

// Block (currently) includes the header, the payload hashes as well as the
// payload contents.
type Block struct {
	Header
	Payload
}

// Valid will check whether the block is valid bottom-up.
func (b Block) Valid() bool {
	return b.PayloadHash == b.Payload.Hash()
}

// Header contains all meta-data for a block, as well as a hash representing
// the combined payload of the entire block. It is what consensus nodes agree
// on after validating the contents against the payload hash.
type Header struct {
	ChainID                 string     // ChainID is a chain-specific value to prevent replay attacks.
	ParentID                Identifier // ParentID is the ID of this block's parent.
	Height                  uint64
	PayloadHash             Identifier // PayloadHash is a hash of the payload of this block.
	Timestamp               time.Time  // Timestamp is the time at which this block was proposed. The proposing node can choose any time, so this should not be trusted as accurate.
	View                    uint64     // View is the view number at which this block was proposed.
	ParentSigners           []Identifier
	ParentStakingSigs       []crypto.Signature // ParentStakingSigs is an aggregated staking signature for the parent block.
	ParentRandomBeaconSig   crypto.Signature   // ParentRandomBeaconSig is a reconstructed signature for random beacon for the parent block.
	ProposerID              Identifier
	ProposerStakingSig      crypto.Signature // ProposerStakingSig is the signature of the proposer over the header body with the staking key.
	ProposerRandomBeaconSig crypto.Signature // ProposerRandomBeaconSig is the signature of the proposer over the header body with the random beacon key share.
}

// Body returns the immutable part of the block header.
func (h Header) Body() interface{} {
	return struct {
		ChainID               string
		ParentID              Identifier
		Height                uint64
		PayloadHash           Identifier
		Timestamp             time.Time
		View                  uint64
		ProposerID            Identifier
		ParentSigners         []Identifier
		ParentStakingSigs     []crypto.Signature
		ParentRandomBeaconSig crypto.Signature
	}{
		ChainID:               h.ChainID,
		ParentID:              h.ParentID,
		Height:                h.Height,
		PayloadHash:           h.PayloadHash,
		Timestamp:             h.Timestamp,
		View:                  h.View,
		ProposerID:            h.ProposerID,
		ParentSigners:         h.ParentSigners,
		ParentStakingSigs:     h.ParentStakingSigs,
		ParentRandomBeaconSig: h.ParentRandomBeaconSig,
	}
}

// ID returns a unique ID to singularly identify the header and its block
// within the flow system.
func (h Header) ID() Identifier {
	return MakeID(h.Body())
}

// Checksum returns the checksum of the header.
func (h Header) Checksum() Identifier {
	return MakeID(h)
}

// Payload is the actual content of each block.
type Payload struct {
	Identities IdentityList
	Guarantees []*CollectionGuarantee
	Seals      []*Seal
}

// Hash returns the root hash of the payload.
func (p Payload) Hash() Identifier {
	idHash := MerkleRoot(GetIDs(p.Identities)...)
	collHash := MerkleRoot(GetIDs(p.Guarantees)...)
	sealHash := MerkleRoot(GetIDs(p.Seals)...)
	return ConcatSum(idHash, collHash, sealHash)
}

// PendingBlock is a wrapper type representing a block that cannot yet be
// processed. The block header, payload, and sender ID are stored together
// while waiting for the block to become processable.
type PendingBlock struct {
	OriginID Identifier
	Header   *Header
	Payload  *Payload
}
