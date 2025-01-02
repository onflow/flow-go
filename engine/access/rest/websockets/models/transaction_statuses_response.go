package models

import (
	"github.com/onflow/flow-go/access"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
)

// Build creates TransactionStatusesResponse instance.
func (t *TransactionStatusesResponse) Build(
	linkGenerator commonmodels.LinkGenerator,
	txResult *access.TransactionResult,
	index uint64,
) {
	var transactionResult commonmodels.TransactionResult
	txID := txResult.TransactionID
	transactionResult.Build(txResult, txID, linkGenerator)

	t.TransactionResult = &transactionResult
	t.MessageIndex = index
}
