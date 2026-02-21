package models

import (
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

const (
	expandableTransaction = "transaction"
	expandableResult      = "result"
)

// Build populates the AccountTransaction from a domain model.
func (t *AccountTransaction) Build(
	tx *accessmodel.AccountTransaction,
	link commonmodels.LinkGenerator,
	expand map[string]bool,
) {
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

	if expand[expandableTransaction] && tx.Transaction != nil {
		t.Transaction = new(commonmodels.Transaction)
		t.Transaction.Build(tx.Transaction, nil, link)
	} else {
		t.Expandable.Transaction = expandableTransaction
	}

	if expand[expandableResult] && tx.Result != nil {
		t.Result = new(commonmodels.TransactionResult)
		t.Result.Build(tx.Result, tx.TransactionID, link)
	} else {
		t.Expandable.Result = expandableResult
	}
}
