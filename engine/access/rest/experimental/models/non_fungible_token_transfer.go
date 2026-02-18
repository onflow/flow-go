package models

import (
	"strconv"

	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	accessmodel "github.com/onflow/flow-go/model/access"
)

// Build populates the NonFungibleTokenTransfer from a domain model.
func (t *NonFungibleTokenTransfer) Build(
	transfer *accessmodel.NonFungibleTokenTransfer,
	link commonmodels.LinkGenerator,
	expand map[string]bool,
) {
	eventIndices := make([]string, len(transfer.EventIndices))
	for i, idx := range transfer.EventIndices {
		eventIndices[i] = strconv.FormatUint(uint64(idx), 10)
	}

	t.BlockHeight = strconv.FormatUint(transfer.BlockHeight, 10)
	t.TransactionId = transfer.TransactionID.String()
	t.TransactionIndex = strconv.FormatUint(uint64(transfer.TransactionIndex), 10)
	t.EventIndices = eventIndices
	t.TokenType = transfer.TokenType
	t.NftId = strconv.FormatUint(transfer.ID, 10)
	t.SourceAddress = transfer.SourceAddress.Hex()
	t.RecipientAddress = transfer.RecipientAddress.Hex()
	t.Expandable = &AccountTransactionExpandable{}

	if expand[expandableTransaction] && transfer.Transaction != nil {
		t.Transaction = new(commonmodels.Transaction)
		t.Transaction.Build(transfer.Transaction, nil, link)
	} else {
		t.Expandable.Transaction = expandableTransaction
	}

	if expand[expandableResult] && transfer.Result != nil {
		t.Result = new(commonmodels.TransactionResult)
		t.Result.Build(transfer.Result, transfer.TransactionID, link)
	} else {
		t.Expandable.Result = expandableResult
	}
}
