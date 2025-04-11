package convert

import (
	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

// TransactionResultToMessage converts a TransactionResult to a protobuf message
func TransactionResultToMessage(result *accessmodel.TransactionResult) *access.TransactionResultResponse {
	return &access.TransactionResultResponse{
		Status:        entities.TransactionStatus(result.Status),
		StatusCode:    uint32(result.StatusCode),
		ErrorMessage:  result.ErrorMessage,
		Events:        EventsToMessages(result.Events),
		BlockId:       result.BlockID[:],
		TransactionId: result.TransactionID[:],
		CollectionId:  result.CollectionID[:],
		BlockHeight:   result.BlockHeight,
	}
}

// MessageToTransactionResult converts a protobuf message to a TransactionResult
func MessageToTransactionResult(message *access.TransactionResultResponse) *accessmodel.TransactionResult {
	return &accessmodel.TransactionResult{
		Status:        flow.TransactionStatus(message.Status),
		StatusCode:    uint(message.StatusCode),
		ErrorMessage:  message.ErrorMessage,
		Events:        MessagesToEvents(message.Events),
		BlockID:       flow.HashToID(message.BlockId),
		TransactionID: flow.HashToID(message.TransactionId),
		CollectionID:  flow.HashToID(message.CollectionId),
		BlockHeight:   message.BlockHeight,
	}
}

// TransactionResultsToMessage converts a slice of TransactionResults to a protobuf message
func TransactionResultsToMessage(results []*accessmodel.TransactionResult) *access.TransactionResultsResponse {
	messages := make([]*access.TransactionResultResponse, len(results))
	for i, result := range results {
		messages[i] = TransactionResultToMessage(result)
	}

	return &access.TransactionResultsResponse{
		TransactionResults: messages,
	}
}

// MessageToTransactionResults converts a protobuf message to a slice of TransactionResults
func MessageToTransactionResults(message *access.TransactionResultsResponse) []*accessmodel.TransactionResult {
	results := make([]*accessmodel.TransactionResult, len(message.TransactionResults))
	for i, result := range message.TransactionResults {
		results[i] = MessageToTransactionResult(result)
	}
	return results
}
