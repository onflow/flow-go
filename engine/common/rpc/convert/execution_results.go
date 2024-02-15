package convert

import (
	"fmt"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/model/flow"
)

// ExecutionResultToMessage converts an execution result to a protobuf message
func ExecutionResultToMessage(er *flow.ExecutionResult) (
	*entities.ExecutionResult,
	error,
) {
	chunks := make([]*entities.Chunk, len(er.Chunks))

	for i, chunk := range er.Chunks {
		chunks[i] = ChunkToMessage(chunk)
	}

	serviceEvents := make([]*entities.ServiceEvent, len(er.ServiceEvents))
	var err error
	for i, serviceEvent := range er.ServiceEvents {
		serviceEvents[i], err = ServiceEventToMessage(serviceEvent)
		if err != nil {
			return nil, fmt.Errorf("error while convering service event %d: %w", i, err)
		}
	}

	return &entities.ExecutionResult{
		PreviousResultId: IdentifierToMessage(er.PreviousResultID),
		BlockId:          IdentifierToMessage(er.BlockID),
		Chunks:           chunks,
		ServiceEvents:    serviceEvents,
		ExecutionDataId:  IdentifierToMessage(er.ExecutionDataID),
	}, nil
}

// MessageToExecutionResult converts a protobuf message to an execution result
func MessageToExecutionResult(m *entities.ExecutionResult) (
	*flow.ExecutionResult,
	error,
) {
	// convert Chunks
	parsedChunks, err := MessagesToChunkList(m.Chunks)
	if err != nil {
		return nil, fmt.Errorf("failed to parse messages to ChunkList: %w", err)
	}
	// convert ServiceEvents
	parsedServiceEvents, err := MessagesToServiceEventList(m.ServiceEvents)
	if err != nil {
		return nil, err
	}
	return &flow.ExecutionResult{
		PreviousResultID: MessageToIdentifier(m.PreviousResultId),
		BlockID:          MessageToIdentifier(m.BlockId),
		Chunks:           parsedChunks,
		ServiceEvents:    parsedServiceEvents,
		ExecutionDataID:  MessageToIdentifier(m.ExecutionDataId),
	}, nil
}

// ExecutionResultsToMessages converts a slice of execution results to a slice of protobuf messages
func ExecutionResultsToMessages(e []*flow.ExecutionResult) (
	[]*entities.ExecutionResult,
	error,
) {
	execResults := make([]*entities.ExecutionResult, len(e))
	for i, execRes := range e {
		parsedExecResult, err := ExecutionResultToMessage(execRes)
		if err != nil {
			return nil, err
		}
		execResults[i] = parsedExecResult
	}
	return execResults, nil
}

// MessagesToExecutionResults converts a slice of protobuf messages to a slice of execution results
func MessagesToExecutionResults(m []*entities.ExecutionResult) (
	[]*flow.ExecutionResult,
	error,
) {
	execResults := make([]*flow.ExecutionResult, len(m))
	for i, e := range m {
		parsedExecResult, err := MessageToExecutionResult(e)
		if err != nil {
			return nil, fmt.Errorf("failed to convert message at index %d to execution result: %w", i, err)
		}
		execResults[i] = parsedExecResult
	}
	return execResults, nil
}

// ExecutionResultMetaListToMessages converts an execution result meta list to a slice of protobuf messages
func ExecutionResultMetaListToMessages(e flow.ExecutionReceiptMetaList) []*entities.ExecutionReceiptMeta {
	messageList := make([]*entities.ExecutionReceiptMeta, len(e))
	for i, execMeta := range e {
		messageList[i] = &entities.ExecutionReceiptMeta{
			ExecutorId:        IdentifierToMessage(execMeta.ExecutorID),
			ResultId:          IdentifierToMessage(execMeta.ResultID),
			Spocks:            SignaturesToMessages(execMeta.Spocks),
			ExecutorSignature: MessageToSignature(execMeta.ExecutorSignature),
		}
	}
	return messageList
}

// MessagesToExecutionResultMetaList converts a slice of protobuf messages to an execution result meta list
func MessagesToExecutionResultMetaList(m []*entities.ExecutionReceiptMeta) flow.ExecutionReceiptMetaList {
	execMetaList := make([]*flow.ExecutionReceiptMeta, len(m))
	for i, message := range m {
		execMetaList[i] = &flow.ExecutionReceiptMeta{
			ExecutorID:        MessageToIdentifier(message.ExecutorId),
			ResultID:          MessageToIdentifier(message.ResultId),
			Spocks:            MessagesToSignatures(message.Spocks),
			ExecutorSignature: MessageToSignature(message.ExecutorSignature),
		}
	}
	return execMetaList[:]
}

// ChunkToMessage converts a chunk to a protobuf message
func ChunkToMessage(chunk *flow.Chunk) *entities.Chunk {
	return &entities.Chunk{
		CollectionIndex:      uint32(chunk.CollectionIndex),
		StartState:           StateCommitmentToMessage(chunk.StartState),
		EventCollection:      IdentifierToMessage(chunk.EventCollection),
		BlockId:              IdentifierToMessage(chunk.BlockID),
		TotalComputationUsed: chunk.TotalComputationUsed,
		NumberOfTransactions: uint32(chunk.NumberOfTransactions),
		Index:                chunk.Index,
		EndState:             StateCommitmentToMessage(chunk.EndState),
	}
}

// MessageToChunk converts a protobuf message to a chunk
func MessageToChunk(m *entities.Chunk) (*flow.Chunk, error) {
	startState, err := flow.ToStateCommitment(m.StartState)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Message start state to Chunk: %w", err)
	}
	endState, err := flow.ToStateCommitment(m.EndState)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Message end state to Chunk: %w", err)
	}
	chunkBody := flow.ChunkBody{
		CollectionIndex:      uint(m.CollectionIndex),
		StartState:           startState,
		EventCollection:      MessageToIdentifier(m.EventCollection),
		BlockID:              MessageToIdentifier(m.BlockId),
		TotalComputationUsed: m.TotalComputationUsed,
		NumberOfTransactions: uint64(m.NumberOfTransactions),
	}
	return &flow.Chunk{
		ChunkBody: chunkBody,
		Index:     m.Index,
		EndState:  endState,
	}, nil
}

// MessagesToChunkList converts a slice of protobuf messages to a chunk list
func MessagesToChunkList(m []*entities.Chunk) (flow.ChunkList, error) {
	parsedChunks := make(flow.ChunkList, len(m))
	for i, chunk := range m {
		parsedChunk, err := MessageToChunk(chunk)
		if err != nil {
			return nil, fmt.Errorf("failed to parse message at index %d to chunk: %w", i, err)
		}
		parsedChunks[i] = parsedChunk
	}
	return parsedChunks, nil
}
