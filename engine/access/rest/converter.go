package rest

import (
	"fmt"

	"github.com/onflow/flow-go/engine/access/rest/generated"
	"github.com/onflow/flow-go/model/flow"
)

// Converter provides the conversion function to/from the Swagger object to Flow objects

func toBlock(flowBlock *flow.Block) *generated.Block {
	return &generated.Block{
		Header:  toBlockHeader(flowBlock.Header),
		Payload: toBlockPayload(flowBlock.Payload),
	}
}

func toBlockHeader(flowHeader *flow.Header) *generated.BlockHeader {
	return &generated.BlockHeader{
		Id:                   flowHeader.ID().String(),
		ParentId:             flowHeader.ParentID.String(),
		Height:               fmt.Sprint(flowHeader.Height),
		Timestamp:            flowHeader.Timestamp,
		ParentVoterSignature: fmt.Sprint(flowHeader.ParentVoterSigData),
	}
}

func toBlockPayload(flowPayload *flow.Payload) *generated.BlockPayload {
	return &generated.BlockPayload{
		CollectionGuarantees: toCollectionGuarantees(flowPayload.Guarantees),
		BlockSeals:           blockSeals(flowPayload.Seals),
	}
}

func toCollectionGuarantees(flowCollGuarantee []*flow.CollectionGuarantee) []generated.CollectionGuarantee {
	collectionGuarantees := make([]generated.CollectionGuarantee, len(flowCollGuarantee))
	for i, flowCollGuarantee := range flowCollGuarantee {
		collectionGuarantees[i] = toCollectionGuarantee(flowCollGuarantee)
	}
	return collectionGuarantees
}

func toCollectionGuarantee(flowCollGuarantee *flow.CollectionGuarantee) generated.CollectionGuarantee {
	signerIDs := make([]string, len(flowCollGuarantee.SignerIDs))
	for i, signerID := range flowCollGuarantee.SignerIDs {
		signerIDs[i] = signerID.String()
	}
	return generated.CollectionGuarantee{
		CollectionId: flowCollGuarantee.CollectionID.String(),
		SignerIds:    signerIDs,
		Signature:    flowCollGuarantee.Signature.String(),
	}
}

func blockSeals(flowSeals []*flow.Seal) []generated.BlockSeal {
	seals := make([]generated.BlockSeal, len(flowSeals))
	for i, seal := range flowSeals {
		seals[i] = blockSeal(seal)
	}
	return seals
}

func blockSeal(flowSeal *flow.Seal) generated.BlockSeal {
	return generated.BlockSeal{
		BlockId:  flowSeal.BlockID.String(),
		ResultId: flowSeal.ResultID.String(),
	}
}
