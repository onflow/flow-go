package models

import (
	"strconv"
	"time"

	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	accessmodel "github.com/onflow/flow-go/model/access"
)

// Build populates the FungibleTokenTransfer from a domain model.
func (t *FungibleTokenTransfer) Build(
	transfer *accessmodel.FungibleTokenTransfer,
	link commonmodels.LinkGenerator,
	expand map[string]bool,
) {
	eventIndices := make([]string, len(transfer.EventIndices))
	for i, idx := range transfer.EventIndices {
		eventIndices[i] = strconv.FormatUint(uint64(idx), 10)
	}

	t.BlockHeight = strconv.FormatUint(transfer.BlockHeight, 10)
	t.BlockTimestamp = time.UnixMilli(int64(transfer.BlockTimestamp)).UTC().Format(time.RFC3339Nano)
	t.TransactionId = transfer.TransactionID.String()
	t.TransactionIndex = strconv.FormatUint(uint64(transfer.TransactionIndex), 10)
	t.EventIndices = eventIndices
	t.TokenType = transfer.TokenType
	t.Amount = transfer.Amount.String()
	t.SourceAddress = transfer.SourceAddress.Hex()
	t.RecipientAddress = transfer.RecipientAddress.Hex()
	t.Expandable = new(AccountTransactionExpandable)

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
