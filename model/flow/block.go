package flow

import (
	"fmt"
	"time"

	"github.com/vmihailenco/msgpack/v4"
)

func Genesis(chainID ChainID) *Block {
	// create the raw content for the genesis block
	payload := Payload{}

	// create the headerBody
	headerBody := HeaderBody{
		ChainID:   chainID,
		ParentID:  ZeroID,
		Height:    0,
		Timestamp: GenesisTime,
		View:      0,
	}

	// combine to block
	return NewRootBlock(
		UntrustedBlock{
			Header:  headerBody,
			Payload: payload,
		},
	)
}

// Block (currently) includes the all block header metadata and the payload content.
//
// Zero values are allowed only for root blocks, which must be constructed
// using the NewRootBlock constructor. All non-root blocks must be constructed
// using NewBlock to ensure validation of the block fields.
//
//structwrite:immutable - mutations allowed only within the constructor
type Block struct {
	// Header is a container encapsulating most of the header fields - *excluding* the payload hash
	// and the proposer signature. Generally, the type [HeaderBody] should not be used on its own.
	// CAUTION regarding security:
	//  * HeaderBody does not contain the hash of the block payload. Therefore, it is not a cryptographic digest
	//    of the block and should not be confused with a "proper" header, which commits to the _entire_ content
	//    of a block.
	//  * With a byzantine HeaderBody alone, an honest node cannot prove who created that faulty data structure,
	//    because HeaderBody does not include the proposer's signature.
	Header  HeaderBody
	Payload Payload
}

// UntrustedBlock is an untrusted input-only representation of a Block,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedBlock should be validated and converted into
// a trusted Block using the NewBlock constructor (or NewRootBlock
// for the root block).
type UntrustedBlock Block

// NewBlock creates a new block.
// This constructor enforces validation rules to ensure the block is well-formed.
// It must be used to construct all non-root blocks.
//
// All errors indicate that a valid Block cannot be constructed from the input.
func NewBlock(untrusted UntrustedBlock) (*Block, error) {
	// validate header body
	untrustedHeaderBody := untrusted.Header
	if untrustedHeaderBody.ParentID == ZeroID {
		return nil, fmt.Errorf("parent ID must not be zero")
	}
	if len(untrustedHeaderBody.ParentVoterIndices) == 0 {
		return nil, fmt.Errorf("parent voter indices must not be empty")
	}
	if len(untrustedHeaderBody.ParentVoterSigData) == 0 {
		return nil, fmt.Errorf("parent voter signature must not be empty")
	}
	if untrustedHeaderBody.ProposerID == ZeroID {
		return nil, fmt.Errorf("proposer ID must not be zero")
	}
	if untrustedHeaderBody.ParentView >= untrustedHeaderBody.View {
		return nil, fmt.Errorf("invalid views - block parent view (%d) is greater than or equal to block view (%d)", untrustedHeaderBody.ParentView, untrustedHeaderBody.View)
	}

	return &Block{
		Header:  untrusted.Header,
		Payload: untrusted.Payload,
	}, nil
}

// NewRootBlock creates a root block.
// This constructor must be used **only** for constructing the root block,
// which is the only case where zero values are allowed.
func NewRootBlock(untrusted UntrustedBlock) *Block {
	return &Block{
		Header:  untrusted.Header,
		Payload: untrusted.Payload,
	}
}

// ID returns a collision-resistant hash of the Block struct.
func (b Block) ID() Identifier {
	return b.ToHeader().ID()
}

// ToHeader converts the block into a compact [flow.Header] representation,
// where the payload is compressed to a hash reference.
func (b Block) ToHeader() *Header {
	return &Header{
		HeaderBody:  b.Header,
		PayloadHash: b.Payload.Hash(),
	}
}

// TODO(malleability): remove MarshalMsgpack when PR #7325 will be merged (convert Header.Timestamp to Unix Milliseconds)
func (b Block) MarshalMsgpack() ([]byte, error) {
	if b.Header.Timestamp.Location() != time.UTC {
		b.Header.Timestamp = b.Header.Timestamp.UTC()
	}

	type Encodable Block
	return msgpack.Marshal(Encodable(b))
}

// TODO(malleability): remove UnmarshalMsgpack when PR #7325 will be merged (convert Header.Timestamp to Unix Milliseconds)
func (b *Block) UnmarshalMsgpack(data []byte) error {
	type Decodable Block
	decodable := Decodable(*b)
	err := msgpack.Unmarshal(data, &decodable)
	*b = Block(decodable)

	if b.Header.Timestamp.Location() != time.UTC {
		b.Header.Timestamp = b.Header.Timestamp.UTC()
	}

	return err
}

// BlockStatus represents the status of a block.
type BlockStatus int

const (
	// BlockStatusUnknown indicates that the block status is not known.
	BlockStatusUnknown BlockStatus = iota
	// BlockStatusFinalized is the status of a finalized block.
	BlockStatusFinalized
	// BlockStatusSealed is the status of a sealed block.
	BlockStatusSealed
)

// String returns the string representation of a transaction status.
func (s BlockStatus) String() string {
	return [...]string{"BLOCK_UNKNOWN", "BLOCK_FINALIZED", "BLOCK_SEALED"}[s]
}

// BlockProposal is a signed proposal that includes the block payload, in addition to the required header and signature.
type BlockProposal struct {
	Block           Block
	ProposerSigData []byte
}

// ProposalHeader converts the proposal into a compact [ProposalHeader] representation,
// where the payload is compressed to a hash reference.
func (b *BlockProposal) ProposalHeader() *ProposalHeader {
	return &ProposalHeader{Header: b.Block.ToHeader(), ProposerSigData: b.ProposerSigData}
}

// CertifiedBlock holds a certified block, which is a block and a Quorum Certificate [QC] pointing
// to the block. A QC is the aggregated form of votes from a supermajority of HotStuff and therefore
// proves validity of the block. A certified block satisfies:
// Block.View == QC.View and Block.BlockID == QC.BlockID
//
// Conceptually, blocks must always be signed by the proposer. Once a block is certified, the
// proposer's signature is included in the QC and does not need to be provided individually anymore.
// Therefore, from the protocol perspective, the canonical data structures are either a block proposal
// (including the proposer's signature) or a certified block (including a QC for the block).
// Though, for simplicity, we just extend the BlockProposal structure to represent a certified block,
// including proof that the proposer has signed their block twice. Thereby it is easy to convert
// a [CertifiedBlock] into a [BlockProposal], which otherwise would not be possible because the QC only
// contains an aggregated signature (including the proposer's signature), which cannot be separated
// into individual signatures.
type CertifiedBlock struct {
	Proposal     *BlockProposal
	CertifyingQC *QuorumCertificate
}

// NewCertifiedBlock constructs a new certified block. It checks the consistency
// requirements and errors otherwise:
//
//	Block.View == QC.View and Block.BlockID == QC.BlockID
func NewCertifiedBlock(proposal *BlockProposal, qc *QuorumCertificate) (CertifiedBlock, error) {
	if proposal.Block.Header.View != qc.View {
		return CertifiedBlock{}, fmt.Errorf("block's view (%d) should equal the qc's view (%d)", proposal.Block.Header.View, qc.View)
	}
	if proposal.Block.ID() != qc.BlockID {
		return CertifiedBlock{}, fmt.Errorf("block's ID (%v) should equal the block referenced by the qc (%d)", proposal.Block.ID(), qc.BlockID)
	}
	return CertifiedBlock{Proposal: proposal, CertifyingQC: qc}, nil
}

// BlockID returns a unique identifier for the block (the ID signed to produce a block vote).
// To avoid repeated computation, we use value from the QC.
// CAUTION: This is not a cryptographic commitment for the CertifiedBlock model.
func (b *CertifiedBlock) BlockID() Identifier {
	return b.CertifyingQC.BlockID
}

// View returns view where the block was produced.
func (b *CertifiedBlock) View() uint64 {
	return b.CertifyingQC.View
}

// Height returns height of the block.
func (b *CertifiedBlock) Height() uint64 {
	return b.Proposal.Block.Header.Height
}

// BlockDigest holds lightweight block information which includes only the block's id, height and timestamp
type BlockDigest struct {
	BlockID   Identifier
	Height    uint64
	Timestamp time.Time
}

// NewBlockDigest constructs a new block digest.
func NewBlockDigest(
	blockID Identifier,
	height uint64,
	timestamp time.Time,
) *BlockDigest {
	return &BlockDigest{
		BlockID:   blockID,
		Height:    height,
		Timestamp: timestamp,
	}
}
