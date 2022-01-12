package models

import (
	"github.com/onflow/flow-go/model/flow"
)

func (b *Block) Build(
	block *flow.Block,
	execResult *flow.ExecutionResult,
	link LinkGenerator,
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
	const ExpandableFieldPayload = "payload"
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
		const ExpandableExecutionResult = "execution_result"
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
	return nil
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

func (b *BlockHeader) Build(header *flow.Header) {
	b.Id = header.ID().String()
	b.ParentId = header.ParentID.String()
	b.Height = fromUint64(header.Height)
	b.Timestamp = header.Timestamp
	b.ParentVoterSignature = ToBase64(header.ParentVoterSigData)
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
	finalState := ""
	if len(seal.FinalState) > 0 { // todo(sideninja) this is always true?
		finalStateBytes, err := seal.FinalState.MarshalJSON()
		if err != nil {
			return err
		}
		finalState = string(finalStateBytes)
	}

	var aggregatedSigs AggregatedSignatures
	aggregatedSigs.Build(seal.AggregatedApprovalSigs)

	b.BlockId = seal.BlockID.String()
	b.ResultId = seal.ResultID.String()
	b.FinalState = finalState
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
		verifierSignatures[y] = ToBase64(verifierSignature.Bytes())
	}

	signerIDs := make([]string, len(signature.SignerIDs))
	for j, signerID := range signature.SignerIDs {
		signerIDs[j] = signerID.String()
	}

	a.VerifierSignatures = verifierSignatures
	a.SignerIds = signerIDs
}
