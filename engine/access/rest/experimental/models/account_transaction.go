package models

import (
	"fmt"
	"strconv"
	"time"

	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

// addressHex returns the hex representation of addr, or an empty string if addr
// is the zero address (flow.EmptyAddress), which indicates a mint or burn.
func addressHex(addr flow.Address) string {
	if addr == flow.EmptyAddress {
		return ""
	}
	return addr.Hex()
}

// Build populates the AccountTransaction from a domain model.
func (t *AccountTransaction) Build(
	tx *accessmodel.AccountTransaction,
	link commonmodels.LinkGenerator,
) error {
	roles := make([]string, len(tx.Roles))
	for i, role := range tx.Roles {
		roles[i] = role.String()
	}

	t.BlockHeight = strconv.FormatUint(tx.BlockHeight, 10)
	t.Timestamp = time.UnixMilli(int64(tx.BlockTimestamp)).UTC().Format(time.RFC3339Nano)
	t.TransactionId = tx.TransactionID.String()
	t.TransactionIndex = strconv.FormatUint(uint64(tx.TransactionIndex), 10)
	t.Roles = roles
	t.Expandable = new(AccountTransactionExpandable)

	if tx.Transaction != nil {
		t.Transaction = new(commonmodels.Transaction)
		t.Transaction.Build(tx.Transaction, nil, link)
	} else {
		transactionLink, err := link.TransactionLink(tx.TransactionID)
		if err != nil {
			return fmt.Errorf("failed to generate transaction link: %w", err)
		}
		t.Expandable.Transaction = transactionLink
	}

	if tx.Result != nil {
		t.Result = new(commonmodels.TransactionResult)
		t.Result.Build(tx.Result, tx.TransactionID, link)
	} else {
		resultLink, err := link.TransactionResultLink(tx.TransactionID)
		if err != nil {
			return fmt.Errorf("failed to generate result link: %w", err)
		}
		t.Expandable.Result = resultLink
	}

	return nil
}
