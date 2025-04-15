package models

import (
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/flow"
)

const ExpandableFieldPayload = "payload"
const ExpandableExecutionResult = "execution_result"

func NewBlock(
	block *flow.Block,
	execResult *flow.ExecutionResult,
	link LinkGenerator,
	blockStatus flow.BlockStatus,
	expand map[string]bool,
) (*Block, error) {
	self, err := SelfLink(block.ID(), link.BlockLink)
	if err != nil {
		return nil, err
	}

	var result Block
	result.Header = NewBlockHeader(block.Header)

	// add the payload to the response if it is specified as an expandable field
	result.Expandable = &BlockExpandable{}
	if expand[ExpandableFieldPayload] {
		var payload BlockPayload
		err := payload.Build(block.Payload)
		if err != nil {
			return nil, err
		}
		result.Payload = &payload
	} else {
		// else add the payload expandable link
		payloadExpandable, err := link.PayloadLink(block.ID())
		if err != nil {
			return nil, err
		}
		result.Expandable.Payload = payloadExpandable
	}

	// execution result might not yet exist
	if execResult != nil {
		// add the execution result to the response if it is specified as an expandable field
		if expand[ExpandableExecutionResult] {
			var exeResult ExecutionResult
			err := exeResult.Build(execResult, link)
			if err != nil {
				return nil, err
			}
			result.ExecutionResult = &exeResult
		} else {
			// else add the execution result expandable link
			executionResultExpandable, err := link.ExecutionResultLink(execResult.ID())
			if err != nil {
				return nil, err
			}
			result.Expandable.ExecutionResult = executionResultExpandable
		}
	}

	result.Links = self

	var status BlockStatus
	status.Build(blockStatus)
	result.BlockStatus = &status

	return &result, nil
}

func (b *Block) Build(
	block *flow.Block,
	execResult *flow.ExecutionResult,
	link LinkGenerator,
	blockStatus flow.BlockStatus,
	expand map[string]bool,
) error {
	self, err := SelfLink(block.ID(), link.BlockLink)
	if err != nil {
		return err
	}

	var header BlockHeader
	header.Build(block.Header)
	b.Header = &header

	// add the payload to the response if it is specified as an expandable field
	b.Expandable = &BlockExpandable{}
	if expand[ExpandableFieldPayload] {
		var payload BlockPayload
		err := payload.Build(block.Payload)
		if err != nil {
			return err
		}
		b.Payload = &payload
	} else {
		// else add the payload expandable link
		payloadExpandable, err := link.PayloadLink(block.ID())
		if err != nil {
			return err
		}
		b.Expandable.Payload = payloadExpandable
	}

	// execution result might not yet exist
	if execResult != nil {
		// add the execution result to the response if it is specified as an expandable field
		if expand[ExpandableExecutionResult] {
			var exeResult ExecutionResult
			err := exeResult.Build(execResult, link)
			if err != nil {
				return err
			}
			b.ExecutionResult = &exeResult
		} else {
			// else add the execution result expandable link
			executionResultExpandable, err := link.ExecutionResultLink(execResult.ID())
			if err != nil {
				return err
			}
			b.Expandable.ExecutionResult = executionResultExpandable
		}
	}

	b.Links = self

	var status BlockStatus
	status.Build(blockStatus)
	b.BlockStatus = &status

	return nil
}

func (b *BlockStatus) Build(status flow.BlockStatus) {
	switch status {
	case flow.BlockStatusUnknown:
		*b = BLOCK_UNKNOWN
	case flow.BlockStatusFinalized:
		*b = BLOCK_FINALIZED
	case flow.BlockStatusSealed:
		*b = BLOCK_SEALED
	default:
		*b = ""
	}
}

func (b *BlockPayload) Build(payload *flow.Payload) error {
	var blockSeal BlockSeals
	err := blockSeal.Build(payload.Seals)
	if err != nil {
		return err
	}
	b.BlockSeals = blockSeal

	var guarantees CollectionGuarantees
	guarantees.Build(payload.Guarantees)
	b.CollectionGuarantees = guarantees

	return nil
}

func NewBlockHeader(header *flow.Header) *BlockHeader {
	return &BlockHeader{
		Id:                   header.ID().String(),
		ParentId:             header.ParentID.String(),
		Height:               util.FromUint(header.Height),
		Timestamp:            header.Timestamp,
		ParentVoterSignature: util.ToBase64(header.ParentVoterSigData),
	}
}

func (b *BlockHeader) Build(header *flow.Header) {
	b.Id = header.ID().String()
	b.ParentId = header.ParentID.String()
	b.Height = util.FromUint(header.Height)
	b.Timestamp = header.Timestamp
	b.ParentVoterSignature = util.ToBase64(header.ParentVoterSigData)
}

type BlockSeals []BlockSeal

func (b *BlockSeals) Build(seals []*flow.Seal) error {
	blkSeals := make([]BlockSeal, len(seals))
	for i, s := range seals {
		var seal BlockSeal
		err := seal.Build(s)
		if err != nil {
			return err
		}
		blkSeals[i] = seal
	}

	*b = blkSeals
	return nil
}

func (b *BlockSeal) Build(seal *flow.Seal) error {
	var aggregatedSigs AggregatedSignatures
	aggregatedSigs.Build(seal.AggregatedApprovalSigs)

	b.BlockId = seal.BlockID.String()
	b.ResultId = seal.ResultID.String()
	b.FinalState = seal.FinalState.String()
	b.AggregatedApprovalSignatures = aggregatedSigs
	return nil
}

type AggregatedSignatures []AggregatedSignature

func (a *AggregatedSignatures) Build(signatures []flow.AggregatedSignature) {
	response := make([]AggregatedSignature, len(signatures))
	for i, signature := range signatures {
		var sig AggregatedSignature
		sig.Build(signature)
		response[i] = sig
	}

	*a = response
}

func (a *AggregatedSignature) Build(signature flow.AggregatedSignature) {
	verifierSignatures := make([]string, len(signature.VerifierSignatures))
	for y, verifierSignature := range signature.VerifierSignatures {
		verifierSignatures[y] = util.ToBase64(verifierSignature.Bytes())
	}

	signerIDs := make([]string, len(signature.SignerIDs))
	for j, signerID := range signature.SignerIDs {
		signerIDs[j] = signerID.String()
	}

	a.VerifierSignatures = verifierSignatures
	a.SignerIds = signerIDs
}
