package status

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
)

type TxStatusDeriver struct {
	state               protocol.State
	lastFullBlockHeight *counters.PersistentStrictMonotonicCounter
}

func NewTxStatusDeriver(state protocol.State, lastFullBlockHeight *counters.PersistentStrictMonotonicCounter) *TxStatusDeriver {
	return &TxStatusDeriver{
		state:               state,
		lastFullBlockHeight: lastFullBlockHeight,
	}
}

// DeriveUnknownTransactionStatus is used to determine the status of transaction who's block is not
// yet known, based on the transaction's reference block.
//
// No errors expected during normal operations.
func (t *TxStatusDeriver) DeriveUnknownTransactionStatus(refBlockID flow.Identifier) (flow.TransactionStatus, error) {
	refBlock, err := t.state.AtBlockID(refBlockID).Head()
	if err != nil {
		return flow.TransactionStatusUnknown, err
	}

	finalized, err := t.state.Final().Head()
	if err != nil {
		return flow.TransactionStatusUnknown, irrecoverable.NewExceptionf("failed to lookup final header: %w", err)
	}

	// if we haven't finalized the expiry block for this transaction, then it is not expired within
	// our view of the chain.
	if !isExpired(refBlock.Height, finalized.Height) {
		return flow.TransactionStatusPending, nil
	}

	// At this point, we have finalized the expiry block for the transaction, which means that
	// if no collections prior to the expiry block contain the transaction, it can never be
	// included and is expired.
	//
	// To confirm this, we need to have received all collections up to the expiry block to ensure
	// the transaction did not appear in any.

	// the last full height is the finalized height up to which we have received all collections
	// for all blocks with lower heights.
	fullHeight := t.lastFullBlockHeight.Value()

	// if we have received collections for all blocks up to the expiry block, the transaction is expired.
	if isExpired(refBlock.Height, fullHeight) {
		return flow.TransactionStatusExpired, nil
	}

	// otherwise, mark it as pending for now until more collections are indexed.
	return flow.TransactionStatusPending, nil
}

// DeriveFinalizedTransactionStatus is used to determine the status of a transaction in a finalized
// block based the block's height and the transaction's executed status.
//
// CAUTION: this must only be used for transactions within FINALIZED blocks, otherwise inaccurate
// results may be returned.
//
// No errors expected during normal operations.
func (t *TxStatusDeriver) DeriveFinalizedTransactionStatus(blockHeight uint64, executed bool) (flow.TransactionStatus, error) {
	if !executed {
		return flow.TransactionStatusFinalized, nil
	}

	sealed, err := t.state.Sealed().Head()
	if err != nil {
		return flow.TransactionStatusUnknown, irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
	}

	// CAUTION: this check is only safe for finalized blocks!
	if blockHeight > sealed.Height {
		return flow.TransactionStatusExecuted, nil
	}

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
