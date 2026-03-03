package models

import (
	"fmt"
	"strconv"
	"time"

	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	accessmodel "github.com/onflow/flow-go/model/access"
)

// Build populates the NonFungibleTokenTransfer from a domain model.
func (t *NonFungibleTokenTransfer) Build(
	transfer *accessmodel.NonFungibleTokenTransfer,
	link commonmodels.LinkGenerator,
) error {
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
	t.NftId = strconv.FormatUint(transfer.ID, 10)
	t.SourceAddress = addressHex(transfer.SourceAddress)
	t.RecipientAddress = addressHex(transfer.RecipientAddress)
	t.Expandable = new(AccountTransactionExpandable)

	if transfer.Transaction != nil {
		t.Transaction = new(commonmodels.Transaction)
		t.Transaction.Build(transfer.Transaction, nil, link)
	} else {
		transactionLink, err := link.TransactionLink(transfer.TransactionID)
		if err != nil {
			return fmt.Errorf("failed to generate transaction link: %w", err)
		}
		t.Expandable.Transaction = transactionLink
	}

	if transfer.Result != nil {
		t.Result = new(commonmodels.TransactionResult)
		t.Result.Build(transfer.Result, transfer.TransactionID, link)
	} else {
		resultLink, err := link.TransactionResultLink(transfer.TransactionID)
		if err != nil {
			return fmt.Errorf("failed to generate result link: %w", err)
		}
		t.Expandable.Result = resultLink
	}

	return nil
}
