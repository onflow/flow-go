package flow

import (
	"fmt"
	"time"
)

// HashablePayload is a temporary interface used to generalize the payload type of GenericBlock.
// It defines the minimal interface required for a payload to participate in block hashing.
//
// TODO(malleability, #7164): remove this interface after renaming IDEntity's method `ID` to `Hash`,
// and replace all usages of HashablePayload with IDEntity.
type HashablePayload interface {
	Hash() Identifier
}

// GenericBlock represents a generic Flow block structure parameterized by a payload type.
// It includes both the block header metadata and the block payload.
//
// Zero values for certain HeaderBody fields are allowed only for root blocks, which must be constructed
// using the NewRootUnsignedBlock constructor. All non-root blocks must be constructed
// using NewUnsignedBlock to ensure validation of the block fields.
//
//structwrite:immutable - mutations allowed only within the constructor
type GenericBlock[T HashablePayload] struct {
	// HeaderBody is a container encapsulating most of the header fields - *excluding* the payload hash
	// and the proposer signature. Generally, the type [HeaderBody] should not be used on its own.
	// CAUTION regarding security:
	//  * HeaderBody does not contain the hash of the block payload. Therefore, it is not a cryptographic digest
	//    of the block and should not be confused with a "proper" header, which commits to the _entire_ content
	//    of a block.
	//  * With a byzantine HeaderBody alone, an honest node cannot prove who created that faulty data structure,
	//    because HeaderBody does not include the proposer's signature.
	HeaderBody
	Payload T
}

// ID returns a collision-resistant hash of the UnsignedBlock struct.
func (b *GenericBlock[T]) ID() Identifier {
	return b.ToHeader().ID()
}

// ToHeader converts the block into a compact [flow.UnsignedHeader] representation,
// where the payload is compressed to a hash reference.
// The receiver UnsignedBlock must be well-formed (enforced by mutation protection on the type).
// This function may panic if invoked on a malformed UnsignedBlock.
func (b *GenericBlock[T]) ToHeader() *UnsignedHeader {
	if !b.ContainsParentQC() {
		rootHeader, err := NewRootUnsignedHeader(UntrustedUnsignedHeader{
			HeaderBody:  b.HeaderBody,
			PayloadHash: b.Payload.Hash(),
		})
		if err != nil {
			panic(fmt.Errorf("could not build root header from block: %w", err))
		}
		return rootHeader
	}

	header, err := NewUnsignedHeader(UntrustedUnsignedHeader{
		HeaderBody:  b.HeaderBody,
		PayloadHash: b.Payload.Hash(),
	})
	if err != nil {
		panic(fmt.Errorf("could not build header from block: %w", err))
	}
	return header
}

// UnsignedBlock is the canonical instantiation of GenericBlock using flow.Payload as the payload type.
//
// Zero values for certain HeaderBody fields are allowed only for root blocks, which must be constructed
// using the NewRootUnsignedBlock constructor. All non-root blocks must be constructed
// using NewUnsignedBlock to ensure validation of the block fields.
//
//structwrite:immutable - mutations allowed only within the constructor
type UnsignedBlock = GenericBlock[Payload]

// UntrustedUnsignedBlock is an untrusted input-only representation of a UnsignedBlock,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedUnsignedBlock should be validated and converted into
// a trusted UnsignedBlock using the NewUnsignedBlock constructor (or NewRootUnsignedBlock
// for the root block).
type UntrustedUnsignedBlock UnsignedBlock

// NewUnsignedBlock creates a new block.
// This constructor enforces validation rules to ensure the block is well-formed.
// It must be used to construct all non-root blocks.
//
// All errors indicate that a valid UnsignedBlock cannot be constructed from the input.
func NewUnsignedBlock(untrusted UntrustedUnsignedBlock) (*UnsignedBlock, error) {
	// validate header body
	headerBody, err := NewHeaderBody(UntrustedHeaderBody(untrusted.HeaderBody))
	if err != nil {
		return nil, fmt.Errorf("invalid header body: %w", err)
	}

	// validate payload
	payload, err := NewPayload(UntrustedPayload(untrusted.Payload))
	if err != nil {
		return nil, fmt.Errorf("invalid payload: %w", err)
	}

	return &UnsignedBlock{
		HeaderBody: *headerBody,
		Payload:    *payload,
	}, nil
}

// NewRootUnsignedBlock creates a root block.
// This constructor must be used **only** for constructing the root block,
// which is the only case where zero values are allowed.
func NewRootUnsignedBlock(untrusted UntrustedUnsignedBlock) (*UnsignedBlock, error) {
	rootHeaderBody, err := NewRootHeaderBody(UntrustedHeaderBody(untrusted.HeaderBody))
	if err != nil {
		return nil, fmt.Errorf("invalid root header body: %w", err)
	}

	// validate payload
	payload, err := NewPayload(UntrustedPayload(untrusted.Payload))
	if err != nil {
		return nil, fmt.Errorf("invalid payload: %w", err)
	}

	return &UnsignedBlock{
		HeaderBody: *rootHeaderBody,
		Payload:    *payload,
	}, nil
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

// Proposal is a signed proposal that includes the block payload, in addition to the required header and signature.
//
//structwrite:immutable - mutations allowed only within the constructor
type Proposal struct {
	Block           UnsignedBlock
	ProposerSigData []byte
}

// UntrustedProposal is an untrusted input-only representation of a Proposal,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedProposal should be validated and converted into
// a trusted Proposal using the NewProposal constructor (or NewRootProposal
// for the root proposal).
type UntrustedProposal Proposal

// NewProposal creates a new Proposal.
// This constructor enforces validation rules to ensure the Proposal is well-formed.
// It must be used to construct all non-root proposal.
//
// All errors indicate that a valid Proposal cannot be constructed from the input.
func NewProposal(untrusted UntrustedProposal) (*Proposal, error) {
	block, err := NewUnsignedBlock(UntrustedUnsignedBlock(untrusted.Block))
	if err != nil {
		return nil, fmt.Errorf("invalid block: %w", err)
	}
	if len(untrusted.ProposerSigData) == 0 {
		return nil, fmt.Errorf("proposer signature must not be empty")
	}

	return &Proposal{
		Block:           *block,
		ProposerSigData: untrusted.ProposerSigData,
	}, nil
}

// NewRootProposal creates a root proposal.
// This constructor must be used **only** for constructing the root proposal,
// which is the only case where zero values are allowed.
func NewRootProposal(untrusted UntrustedProposal) (*Proposal, error) {
	block, err := NewRootUnsignedBlock(UntrustedUnsignedBlock(untrusted.Block))
	if err != nil {
		return nil, fmt.Errorf("invalid root block: %w", err)
	}
	if len(untrusted.ProposerSigData) > 0 {
		return nil, fmt.Errorf("proposer signature must be empty")
	}

	return &Proposal{
		Block:           *block,
		ProposerSigData: untrusted.ProposerSigData,
	}, nil
}

// ProposalHeader converts the proposal into a compact [ProposalHeader] representation,
// where the payload is compressed to a hash reference.
func (b *Proposal) ProposalHeader() *ProposalHeader {
	block := &b.Block
	return &ProposalHeader{Header: block.ToHeader(), ProposerSigData: b.ProposerSigData}
}

// CertifiedBlock holds a certified block, which is a block and a Quorum Certificate [QC] pointing
// to the block. A QC is the aggregated form of votes from a supermajority of HotStuff and therefore
// proves validity of the block. A certified block satisfies:
// UnsignedBlock.View == QC.View and UnsignedBlock.BlockID == QC.BlockID
//
// Conceptually, blocks must always be signed by the proposer. Once a block is certified, the
// proposer's signature is included in the QC and does not need to be provided individually anymore.
// Therefore, from the protocol perspective, the canonical data structures are either a block proposal
// (including the proposer's signature) or a certified block (including a QC for the block).
// Though, for simplicity, we just extend the Proposal structure to represent a certified block,
// including proof that the proposer has signed their block twice. Thereby it is easy to convert
// a [CertifiedBlock] into a [Proposal], which otherwise would not be possible because the QC only
// contains an aggregated signature (including the proposer's signature), which cannot be separated
// into individual signatures.
type CertifiedBlock struct {
	Proposal     *Proposal
	CertifyingQC *QuorumCertificate
}

// NewCertifiedBlock constructs a new certified block. It checks the consistency
// requirements and errors otherwise:
//
//	UnsignedBlock.View == QC.View and UnsignedBlock.BlockID == QC.BlockID
func NewCertifiedBlock(proposal *Proposal, qc *QuorumCertificate) (CertifiedBlock, error) {
	if proposal.Block.View != qc.View {
		return CertifiedBlock{}, fmt.Errorf("block's view (%d) should equal the qc's view (%d)", proposal.Block.View, qc.View)
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
	return b.Proposal.Block.Height
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
