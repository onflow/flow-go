package convert

import (
	"fmt"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/onflow/flow-go/model/flow"

	"github.com/onflow/flow/protobuf/go/flow/entities"
)

// BlockToMessage converts a flow.Block to a protobuf Block message.
// signerIDs is a precomputed list of signer IDs for the block based on the block's signer indicies.
func BlockToMessage(h *flow.Block, signerIDs flow.IdentifierList) (
	*entities.Block,
	error,
) {
	id := h.ID()

	parentID := h.Header.ParentID
	t := timestamppb.New(h.Header.Timestamp)
	cg := CollectionGuaranteesToMessages(h.Payload.Guarantees)

	seals := BlockSealsToMessages(h.Payload.Seals)

	execResults, err := ExecutionResultsToMessages(h.Payload.Results)
	if err != nil {
		return nil, err
	}

	blockHeader, err := BlockHeaderToMessage(h.Header, signerIDs)
	if err != nil {
		return nil, err
	}

	bh := entities.Block{
		Id:                       id[:],
		Height:                   h.Header.Height,
		ParentId:                 parentID[:],
		Timestamp:                t,
		CollectionGuarantees:     cg,
		BlockSeals:               seals,
		Signatures:               [][]byte{h.Header.ParentVoterSigData},
		ExecutionReceiptMetaList: ExecutionResultMetaListToMessages(h.Payload.Receipts),
		ExecutionResultList:      execResults,
		BlockHeader:              blockHeader,
	}

	return &bh, nil
}

// BlockToMessageLight converts a flow.Block to the light form of a protobuf Block message.
func BlockToMessageLight(h *flow.Block) *entities.Block {
	id := h.ID()

	parentID := h.Header.ParentID
	t := timestamppb.New(h.Header.Timestamp)
	cg := CollectionGuaranteesToMessages(h.Payload.Guarantees)

	return &entities.Block{
		Id:                   id[:],
		Height:               h.Header.Height,
		ParentId:             parentID[:],
		Timestamp:            t,
		CollectionGuarantees: cg,
		Signatures:           [][]byte{h.Header.ParentVoterSigData},
	}
}

// MessageToBlock converts a protobuf Block message to a flow.Block.
func MessageToBlock(m *entities.Block) (*flow.Block, error) {
	payload, err := PayloadFromMessage(m)
	if err != nil {
		return nil, fmt.Errorf("failed to extract payload data from message: %w", err)
	}
	header, err := MessageToBlockHeader(m.BlockHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to convert block header: %w", err)
	}
	return &flow.Block{
		Header:  header,
		Payload: payload,
	}, nil
}

// BlockSealToMessage converts a flow.Seal to a protobuf BlockSeal message.
func BlockSealToMessage(s *flow.Seal) *entities.BlockSeal {
	id := s.BlockID
	result := s.ResultID
	return &entities.BlockSeal{
		BlockId:                    id[:],
		ExecutionReceiptId:         result[:],
		ExecutionReceiptSignatures: [][]byte{}, // filling seals signature with zero
		FinalState:                 StateCommitmentToMessage(s.FinalState),
		AggregatedApprovalSigs:     AggregatedSignaturesToMessages(s.AggregatedApprovalSigs),
		ResultId:                   IdentifierToMessage(s.ResultID),
	}
}

// MessageToBlockSeal converts a protobuf BlockSeal message to a flow.Seal.
func MessageToBlockSeal(m *entities.BlockSeal) (*flow.Seal, error) {
	finalState, err := MessageToStateCommitment(m.FinalState)
	if err != nil {
		return nil, fmt.Errorf("failed to convert message to block seal: %w", err)
	}
	return &flow.Seal{
		BlockID:                MessageToIdentifier(m.BlockId),
		ResultID:               MessageToIdentifier(m.ResultId),
		FinalState:             finalState,
		AggregatedApprovalSigs: MessagesToAggregatedSignatures(m.AggregatedApprovalSigs),
	}, nil
}

// BlockSealsToMessages converts a slice of flow.Seal to a slice of protobuf BlockSeal messages.
func BlockSealsToMessages(b []*flow.Seal) []*entities.BlockSeal {
	seals := make([]*entities.BlockSeal, len(b))
	for i, s := range b {
		seals[i] = BlockSealToMessage(s)
	}
	return seals
}

// MessagesToBlockSeals converts a slice of protobuf BlockSeal messages to a slice of flow.Seal.
func MessagesToBlockSeals(m []*entities.BlockSeal) ([]*flow.Seal, error) {
	seals := make([]*flow.Seal, len(m))
	for i, s := range m {
		msg, err := MessageToBlockSeal(s)
		if err != nil {
			return nil, err
		}
		seals[i] = msg
	}
	return seals, nil
}

// PayloadFromMessage converts a protobuf Block message to a flow.Payload.
func PayloadFromMessage(m *entities.Block) (*flow.Payload, error) {
	cgs := MessagesToCollectionGuarantees(m.CollectionGuarantees)
	seals, err := MessagesToBlockSeals(m.BlockSeals)
	if err != nil {
		return nil, err
	}
	receipts := MessagesToExecutionResultMetaList(m.ExecutionReceiptMetaList)
	results, err := MessagesToExecutionResults(m.ExecutionResultList)
	if err != nil {
		return nil, err
	}
	return &flow.Payload{
		Guarantees: cgs,
		Seals:      seals,
		Receipts:   receipts,
		Results:    results,
	}, nil
}

//// BlockStatusToMessage converts a flow.BlockStatus to a protobuf BlockStatus message.
//func BlockStatusToMessage(status flow.BlockStatus) entities.BlockStatus {
//	switch status {
//	case flow.BlockStatusUnknown:
//		return entities.BlockStatus_BLOCK_UNKNOWN
//	case flow.BlockStatusFinalized:
//		return entities.BlockStatus_BLOCK_FINALIZED
//	case flow.BlockStatusSealed:
//		return entities.BlockStatus_BLOCK_SEALED
//	}
//	return entities.BlockStatus_BLOCK_UNKNOWN
//}

// MessageToBlockStatus converts a protobuf BlockStatus message to a flow.BlockStatus.
func MessageToBlockStatus(status entities.BlockStatus) flow.BlockStatus {
	switch status {
	case entities.BlockStatus_BLOCK_UNKNOWN:
		return flow.BlockStatusUnknown
	case entities.BlockStatus_BLOCK_FINALIZED:
		return flow.BlockStatusFinalized
	case entities.BlockStatus_BLOCK_SEALED:
		return flow.BlockStatusSealed
	}
	return flow.BlockStatusUnknown
}
