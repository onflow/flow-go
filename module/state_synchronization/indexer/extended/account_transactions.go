package extended

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

const accountTransactionsIndexerName = "account_transactions"

// AccountTransactions indexes account-transaction associations for a block.
type AccountTransactions struct {
	log         zerolog.Logger
	store       storage.AccountTransactionsBootstrapper
	chainID     flow.ChainID
	lockManager storage.LockManager
}

var _ Indexer = (*AccountTransactions)(nil)

func NewAccountTransactions(
	log zerolog.Logger,
	store storage.AccountTransactionsBootstrapper,
	chainID flow.ChainID,
	lockManager storage.LockManager,
) *AccountTransactions {
	return &AccountTransactions{
		log:         log.With().Str("component", "account_transactions_indexer").Logger(),
		store:       store,
		chainID:     chainID,
		lockManager: lockManager,
	}
}

// Name returns the name of the indexer.
func (a *AccountTransactions) Name() string {
	return accountTransactionsIndexerName
}

// NextHeight returns the next height that the indexer will index.
//
// No error returns are expected during normal operation.
func (a *AccountTransactions) NextHeight() (uint64, error) {
	height, err := a.store.LatestIndexedHeight()
	if err == nil {
		return height + 1, nil
	}

	if !errors.Is(err, storage.ErrNotBootstrapped) {
		return 0, fmt.Errorf("failed to get latest indexed height: %w", err)
	}

	firstHeight, isInitialized := a.store.UninitializedFirstHeight()
	if isInitialized {
		// this shouldn't happen and would indicate a bug or inconsistent state.
		return 0, fmt.Errorf("failed to get latest indexed height, but index is initialized: %w", err)
	}

	return firstHeight, nil
}

// IndexBlockData indexes the block data for the given height.
// If the header in `data` does not match the expected height, an error is returned.
//
// The caller must hold the [storage.LockIndexAccountTransactions] lock until the batch is committed.
//
// Not safe for concurrent use.
//
// Expected error returns during normal operations:
//   - [ErrAlreadyIndexed]: if the data is already indexed for the height.
//   - [ErrFutureHeight]: if the data is for a future height.
func (a *AccountTransactions) IndexBlockData(lctx lockctx.Proof, data BlockData, batch storage.ReaderBatchWriter) error {
	expectedHeight, err := a.NextHeight()
	if err != nil {
		return fmt.Errorf("failed to get latest indexed height: %w", err)
	}
	if data.Header.Height > expectedHeight {
		return ErrFutureHeight
	}
	if data.Header.Height < expectedHeight {
		return ErrAlreadyIndexed
	}

	entries, err := a.buildAccountTransactionsFromBlockData(data)
	if err != nil {
		return fmt.Errorf("failed to build account transactions from block data: %w", err)
	}

	if err := a.store.Store(lctx, batch, data.Header.Height, entries); err != nil {
		// since we have already checked that the height is not already indexed, no errors are expected
		// here and indicate concurrent indexing which is not supported.
		return fmt.Errorf("failed to store account transactions: %w", err)
	}

	return nil
}

func (a *AccountTransactions) buildAccountTransactionsFromBlockData(data BlockData) ([]access.AccountTransaction, error) {
	chain := a.chainID.Chain()
	entries := make([]access.AccountTransaction, 0)
	for i, tx := range data.Transactions {
		txIndex := uint32(i)
		addresses := make(map[flow.Address]bool)
		authorizers := make(map[flow.Address]bool)

		addresses[tx.Payer] = true
		addresses[tx.ProposalKey.Address] = true
		for _, auth := range tx.Authorizers {
			addresses[auth] = true
			authorizers[auth] = true
		}

		for _, event := range data.Events[txIndex] {
			eventAddresses, err := a.extractAddresses(event)
			if err != nil {
				return nil, fmt.Errorf("failed to extract addresses from event: %w", err)
			}
			for _, addr := range eventAddresses {
				addresses[addr] = true
			}
		}

		for addr := range addresses {
			// since `extractAddresses` returns all [cadence.Address] fields in the event, it's possible
			// that the event contains invalid addresses, or addresses from a different chain.
			// Only index addresses that are actually valid for the current chain.
			if chain.IsValid(addr) {
				entries = append(entries, access.AccountTransaction{
					Address:          addr,
					BlockHeight:      data.Header.Height,
					TransactionID:    tx.ID(),
					TransactionIndex: txIndex,
					IsAuthorizer:     authorizers[addr],
				})
			}
		}
	}
	return entries, nil
}

// extractAddresses extracts all addresses referenced in a flow event.
//
// No error returns are expected during normal operation.
func (a *AccountTransactions) extractAddresses(event flow.Event) ([]flow.Address, error) {
	cadenceEvent, err := decodeEventPayload(event.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode event payload: %w", err)
	}

	addresses := make([]flow.Address, 0)

	fields := cadence.FieldsMappedByName(cadenceEvent)
	for _, field := range fields {
		switch v := field.(type) {
		case cadence.Address:
			addresses = append(addresses, flow.Address(v))
		case cadence.Optional:
			if v.Value == nil {
				continue
			}

			addr, ok := v.Value.(cadence.Address)
			if !ok {
				continue
			}

			addresses = append(addresses, flow.Address(addr))
		}
	}
	return addresses, nil
}

// decodeEventPayload decodes CCF-encoded event payload.
//
// Any error indicates that the event payload is malformed.
func decodeEventPayload(payload []byte) (cadence.Event, error) {
	value, err := ccf.Decode(nil, payload)
	if err != nil {
		return cadence.Event{}, fmt.Errorf("failed to decode CCF payload: %w", err)
	}

	event, ok := value.(cadence.Event)
	if !ok {
		return cadence.Event{}, fmt.Errorf("decoded value is not an event: %T", value)
	}

	return event, nil
}
