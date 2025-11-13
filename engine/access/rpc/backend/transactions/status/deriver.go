package status

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
)

type TxStatusDeriver struct {
	state               protocol.State
	lastFullBlockHeight counters.Reader
}

func NewTxStatusDeriver(state protocol.State, lastFullBlockHeight counters.Reader) *TxStatusDeriver {
	return &TxStatusDeriver{
		state:               state,
		lastFullBlockHeight: lastFullBlockHeight,
	}
}

// DeriveUnknownTransactionStatus is used to determine the status of transaction
// that are not in a block yet based on the provided reference block ID.
func (t *TxStatusDeriver) DeriveUnknownTransactionStatus(refBlockID flow.Identifier) (flow.TransactionStatus, error) {
	referenceBlock, err := t.state.AtBlockID(refBlockID).Head()
	if err != nil {
		return flow.TransactionStatusUnknown, err
	}
	refHeight := referenceBlock.Height
	// get the latest finalized block from the state
	finalized, err := t.state.Final().Head()
	if err != nil {
		return flow.TransactionStatusUnknown, irrecoverable.NewExceptionf("failed to lookup final header: %w", err)
	}
	finalizedHeight := finalized.Height

	// if we haven't seen the expiry block for this transaction, it's not expired
	if !isExpired(refHeight, finalizedHeight) {
		return flow.TransactionStatusPending, nil
	}

	// At this point, we have seen the expiry block for the transaction.
	// This means that, if no collections  prior to the expiry block contain
	// the transaction, it can never be included and is expired.
	//
	// To ensure this, we need to have received all collections  up to the
	// expiry block to ensure the transaction did not appear in any.

	// the last full height is the height where we have received all
	// collections  for all blocks with a lower height
	fullHeight := t.lastFullBlockHeight.Value()

	// if we have received collections  for all blocks up to the expiry block, the transaction is expired
	if isExpired(refHeight, fullHeight) {
		return flow.TransactionStatusExpired, nil
	}

	// tx found in transaction storage and collection storage but not in block storage
	// However, this will not happen as of now since the ingestion engine doesn't subscribe
	// for collections
	return flow.TransactionStatusPending, nil
}

// DeriveTransactionStatus is used to determine the status of a transaction based on the provided block height, and execution status.
// No errors expected during normal operations.
func (t *TxStatusDeriver) DeriveTransactionStatus(blockHeight uint64, executed bool) (flow.TransactionStatus, error) {
	if !executed {
		// If we've gotten here, but the block has not yet been executed, report it as only been finalized
		return flow.TransactionStatusFinalized, nil
	}

	// From this point on, we know for sure this transaction has at least been executed

	// get the latest sealed block from the State
	sealed, err := t.state.Sealed().Head()
	if err != nil {
		return flow.TransactionStatusUnknown, irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
	}

	if blockHeight > sealed.Height {
		// The block is not yet sealed, so we'll report it as only executed
		return flow.TransactionStatusExecuted, nil
	}

	// otherwise, this block has been executed, and sealed, so report as sealed
	return flow.TransactionStatusSealed, nil
}

// isExpired checks whether a transaction is expired given the height of the
// transaction's reference block and the height to compare against.
func isExpired(refHeight, compareToHeight uint64) bool {
	if compareToHeight <= refHeight {
		return false
	}
	return compareToHeight-refHeight > flow.DefaultTransactionExpiry
}
