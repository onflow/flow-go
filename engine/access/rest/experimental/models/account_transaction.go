package models

import (
	"strconv"

	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	accessmodel "github.com/onflow/flow-go/model/access"
)

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
	t.TransactionId = tx.TransactionID.String()
	t.TransactionIndex = strconv.FormatUint(uint64(tx.TransactionIndex), 10)
	t.Roles = roles
	t.Expandable = &AccountTransactionExpandable{}

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
