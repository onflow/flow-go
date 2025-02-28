package models

import (
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	accessmodel "github.com/onflow/flow-go/model/access"
)

// Build creates TransactionStatusesResponse instance.
func (t *TransactionStatusesResponse) Build(
	linkGenerator commonmodels.LinkGenerator,
	txResult *accessmodel.TransactionResult,
	index uint64,
) {
	var transactionResult commonmodels.TransactionResult
	txID := txResult.TransactionID
	transactionResult.Build(txResult, txID, linkGenerator)

	t.TransactionResult = &transactionResult
	t.MessageIndex = index
}
