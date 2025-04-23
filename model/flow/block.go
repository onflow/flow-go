package flow

import (
	"fmt"
	"time"
)

func Genesis(chainID ChainID) *Block {

	// create the raw content for the genesis block
	payload := Payload{}

	// create the header
	header := Header{
		ChainID:     chainID,
		ParentID:    ZeroID,
		Height:      0,
		PayloadHash: payload.Hash(),
		Timestamp:   uint64(GenesisTime.UnixMilli()),
		View:        0,
	}

	// combine to block
	genesis := Block{
		Header:  &header,
		Payload: &payload,
	}

	return &genesis
}

// Block (currently) includes the header, the payload hashes as well as the
// payload contents.
type Block struct {
	Header  *Header
	Payload *Payload
}

// SetPayload sets the payload and updates the payload hash.
func (b *Block) SetPayload(payload Payload) {
	b.Payload = &payload
	b.Header.PayloadHash = b.Payload.Hash()
}

// Valid will check whether the block is valid bottom-up.
func (b Block) Valid() bool {
	return b.Header.PayloadHash == b.Payload.Hash()
}

// ID returns the ID of the header.
func (b Block) ID() Identifier {
	return b.Header.ID()
}

// Checksum returns the checksum of the header.
func (b Block) Checksum() Identifier {
	return b.Header.Checksum()
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
	Block           *Block
	ProposerSigData []byte
}

func (b *BlockProposal) HeaderProposal() *ProposalHeader {
	return &ProposalHeader{Header: b.Block.Header, ProposerSigData: b.ProposerSigData}
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
