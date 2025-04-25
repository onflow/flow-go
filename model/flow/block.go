package flow

import (
	"fmt"
	"io"
	"time"

	"github.com/onflow/go-ethereum/rlp"
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
	genesis := Block{
		Header:  &headerBody,
		Payload: &payload,
	}

	return &genesis
}

// Block (currently) includes the header, the payload hashes as well as the
// payload contents.
type Block struct {
	Header  *HeaderBody
	Payload *Payload
}

// NewBlock creates a new block.
//
// Parameters:
// - headerBody: the header fields to use for the block
// - payload: the payload to associate with the block
func NewBlock(
	headerBody HeaderBody,
	payload Payload,
) *Block {
	return &Block{
		Header:  &headerBody,
		Payload: &payload,
	}
}

// SetPayload sets the payload and updates the payload hash.
func (b *Block) SetPayload(payload Payload) {
	b.Payload = &payload
	//b.Header.PayloadHash = b.Payload.Hash()
}

func (b *Block) ID() Identifier {
	return MakeID(b)
}

// EncodeRLP defines custom encoding for the Block to calculate its ID.
// The hash of the block is not just the hash of all the fields, It's a two-step process.
// If we just hash of all the fields, we lose the ability to have like a compressed data structure like the header.
// We first hash the payload fields, and then with that hash of the payload fields, we hash the header body fields and include the hash of the payload.
// This convention ensures that both the header and the block produce the same hash.
// The Timestamp is converted from time.Time to Unix time (uint64), which is necessary
// because time.Time is not RLP-encodable due to its private fields.
func (b *Block) EncodeRLP(w io.Writer) error {
	payloadHash := b.Payload.Hash()

	// the order of the fields is kept according to the flow.Header Fingerprint()
	encodingCanonicalForm := struct {
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
		ChainID:            b.Header.ChainID,
		ParentID:           b.Header.ParentID,
		Height:             b.Header.Height,
		PayloadHash:        payloadHash,
		Timestamp:          uint64(b.Header.Timestamp.UnixNano()),
		View:               b.Header.View,
		ParentView:         b.Header.ParentView,
		ParentVoterIndices: b.Header.ParentVoterIndices,
		ParentVoterSigData: b.Header.ParentVoterSigData,
		ProposerID:         b.Header.ProposerID,
		LastViewTCID:       b.Header.LastViewTC.ID(),
	}

	return rlp.Encode(w, encodingCanonicalForm)
}

// ToHeader return flow.Header data for Block.
func (b *Block) ToHeader() *Header {
	return &Header{
		HeaderBody:  *b.Header,
		PayloadHash: b.Payload.Hash(),
	}
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
