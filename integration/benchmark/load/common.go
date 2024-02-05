package load

import (
	"errors"
	"fmt"
	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/integration/benchmark/common"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/integration/benchmark/account"
)

// transactionFunc is a function that creates a transaction.
// It is used by sendSimpleTransaction.
type transactionFunc func(
	log zerolog.Logger,
	lc LoadContext,
	acc *account.FlowAccount,
) (*flowsdk.Transaction, error)

// sendSimpleTransaction is a helper function for sending a transaction.
// It
// - borrows an account,
// - creates a transaction,
// - sets the reference block ID,
// - sets the proposer and payer and one authorizer (if not already set),
// - signs it with the account,
// - sends the transaction to the network.
// - waits for the transaction result.
func sendSimpleTransaction(log zerolog.Logger, lc LoadContext, txFN transactionFunc) error {
	wrapErr := func(err error) error {
		return fmt.Errorf("error in send simple transaction: %w", err)
	}

	acc, err := lc.BorrowAvailableAccount()
	if err != nil {
		return wrapErr(err)
	}
	defer lc.ReturnAvailableAccount(acc)

	tx, err := txFN(log, lc, acc)
	if err != nil {
		return wrapErr(err)
	}

	tx.SetReferenceBlockID(lc.ReferenceBlockID())

	key, err := acc.GetKey()
	if err != nil {
		return wrapErr(err)
	}
	defer key.Done()

	err = key.SetProposerPayerAndSign(tx)
	if err != nil {
		return wrapErr(err)
	}

	_, err = lc.Send(tx)
	if err == nil || !errors.Is(err, common.TransactionError{}) {
		key.IncrementSequenceNumber()
	}
	if err != nil {

		return wrapErr(err)
	}

	return nil
}
