package models

import (
	"github.com/onflow/flow-go/access"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
)

// Build creates TransactionStatusesResponse instance.
func (t *TransactionStatusesResponse) Build(
	linkGenerator commonmodels.LinkGenerator,
	txResults []*access.TransactionResult,
	index uint64,
) {
	transactionResults := make([]*commonmodels.TransactionResult, len(txResults))
	for i, txResult := range txResults {
		var transactionResult commonmodels.TransactionResult
		txID := txResult.TransactionID
		transactionResult.Build(txResult, txID, linkGenerator)

		transactionResults[i] = &transactionResult
	}

	t.TransactionResults = transactionResults
	t.MessageIndex = index
}
