package models

import (
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	accessmodel "github.com/onflow/flow-go/model/access"
)

// TransactionStatusesResponse is the response message for 'events' topic.
type TransactionStatusesResponse struct {
	TransactionResult *commonmodels.TransactionResult `json:"transaction_result"`
	MessageIndex      uint64                          `json:"message_index"`
}

// NewTransactionStatusesResponse creates a TransactionStatusesResponse instance.
func NewTransactionStatusesResponse(
	linkGenerator commonmodels.LinkGenerator,
	txResult *accessmodel.TransactionResult,
	index uint64,
) *TransactionStatusesResponse {
	var transactionResult commonmodels.TransactionResult
	txID := txResult.TransactionID
	transactionResult.Build(txResult, txID, linkGenerator)

	return &TransactionStatusesResponse{
		TransactionResult: &transactionResult,
		MessageIndex:      index,
	}
}
