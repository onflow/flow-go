package convert

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/onflow/flow-go/model/flow"

	"github.com/onflow/flow/protobuf/go/flow/entities"
)

// BlockTimestamp2ProtobufTime is just a shorthand function to ensure consistent conversion
// of block timestamps (measured in unix milliseconds) to protobuf's Timestamp format.
func BlockTimestamp2ProtobufTime(blockTimestamp uint64) *timestamppb.Timestamp {
	return timestamppb.New(time.UnixMilli(int64(blockTimestamp)))
}

// BlockToMessage converts a flow.Block to a protobuf Block message.
// signerIDs is a precomputed list of signer IDs for the block based on the block's signer indices.
func BlockToMessage(h *flow.Block, signerIDs flow.IdentifierList) (
	*entities.Block,
	error,
) {
	id := h.ID()
	cg := CollectionGuaranteesToMessages(h.Payload.Guarantees)
	seals := BlockSealsToMessages(h.Payload.Seals)

	execResults, err := ExecutionResultsToMessages(h.Payload.Results)
	if err != nil {
		return nil, err
	}

	blockHeader, err := BlockHeaderToMessage(h.ToHeader(), signerIDs)
	if err != nil {
		return nil, err
	}

	bh := entities.Block{
		Id:                       IdentifierToMessage(id),
		Height:                   h.Height,
		ParentId:                 IdentifierToMessage(h.ParentID),
		Timestamp:                BlockTimestamp2ProtobufTime(h.Timestamp),
		CollectionGuarantees:     cg,
		BlockSeals:               seals,
		Signatures:               [][]byte{h.ParentVoterSigData},
		ExecutionReceiptMetaList: ExecutionResultMetaListToMessages(h.Payload.Receipts),
		ExecutionResultList:      execResults,
		ProtocolStateId:          IdentifierToMessage(h.Payload.ProtocolStateID),
		BlockHeader:              blockHeader,
	}
	return &bh, nil
}

// BlockToMessageLight converts a flow.Block to the light form of a protobuf Block message.
func BlockToMessageLight(h *flow.Block) *entities.Block {
	id := h.ID()
	cg := CollectionGuaranteesToMessages(h.Payload.Guarantees)

	return &entities.Block{
		Id:                   id[:],
		Height:               h.Height,
		ParentId:             h.ParentID[:],
		Timestamp:            BlockTimestamp2ProtobufTime(h.Timestamp),
		CollectionGuarantees: cg,
		Signatures:           [][]byte{h.ParentVoterSigData},
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

	if IsRootBlockHeader(m.BlockHeader) {
		block, err := flow.NewRootBlock(
			flow.UntrustedBlock{
				HeaderBody: header.HeaderBody,
				Payload:    *payload,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create root block: %w", err)
		}
		return block, nil
	}

	block, err := flow.NewBlock(
		flow.UntrustedBlock{
			HeaderBody: header.HeaderBody,
			Payload:    *payload,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("could not build block: %w", err)
	}

	return block, nil
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
//
// All errors indicate the input cannot be converted to a valid seal.
func MessageToBlockSeal(m *entities.BlockSeal) (*flow.Seal, error) {
	finalState, err := MessageToStateCommitment(m.FinalState)
	if err != nil {
		return nil, fmt.Errorf("failed to convert message to block seal: %w", err)
	}
	seal, err := flow.NewSeal(
		flow.UntrustedSeal{
			BlockID:                MessageToIdentifier(m.BlockId),
			ResultID:               MessageToIdentifier(m.ResultId),
			FinalState:             finalState,
			AggregatedApprovalSigs: MessagesToAggregatedSignatures(m.AggregatedApprovalSigs),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("could not construct seal: %w", err)
	}

	return seal, nil
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
	cgs, err := MessagesToCollectionGuarantees(m.CollectionGuarantees)
	if err != nil {
		return nil, err
	}
	seals, err := MessagesToBlockSeals(m.BlockSeals)
	if err != nil {
		return nil, err
	}
	receipts, err := MessagesToExecutionResultMetaList(m.ExecutionReceiptMetaList)
	if err != nil {
		return nil, err
	}
	results, err := MessagesToExecutionResults(m.ExecutionResultList)
	if err != nil {
		return nil, err
	}
	payload, err := flow.NewPayload(
		flow.UntrustedPayload{
			Guarantees:      cgs,
			Seals:           seals,
			Receipts:        receipts,
			Results:         results,
			ProtocolStateID: MessageToIdentifier(m.ProtocolStateId),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("could not build the payload: %w", err)
	}

	return payload, nil
}

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
