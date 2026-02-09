package extended

import (
	"fmt"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/account_data/transfers"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

const accountTransactionsIndexerName = "account_transactions"

// AccountTransactions indexes account-transaction associations for a block.
type AccountTransactions struct {
	log            zerolog.Logger
	store          storage.AccountTransactions
	chainID        flow.ChainID
	transferParser *transfers.Parser
	lockManager    storage.LockManager
}

var _ Indexer = (*AccountTransactions)(nil)

func NewAccountTransactions(
	log zerolog.Logger,
	store storage.AccountTransactions,
	chainID flow.ChainID,
	lockManager storage.LockManager,
) *AccountTransactions {
	return &AccountTransactions{
		log:            log.With().Str("component", "account_transactions_indexer").Logger(),
		store:          store,
		chainID:        chainID,
		transferParser: transfers.NewParser(),
		lockManager:    lockManager,
	}
}

// Name returns the name of the indexer.
func (a *AccountTransactions) Name() string {
	return accountTransactionsIndexerName
}

// LatestIndexedHeight returns the latest indexed height for the indexer.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotFound]: if the index has not been initialized
func (a *AccountTransactions) LatestIndexedHeight() (uint64, error) {
	return a.store.LatestIndexedHeight()
}

// IndexBlockData indexes the block data for the given height.
//
// Expected error returns during normal operations:
//   - [ErrAlreadyIndexed]: if the data is already indexed for the height.
//   - [ErrFutureHeight]: if the data is for a future height.
func (a *AccountTransactions) IndexBlockData(lctx lockctx.Proof, data BlockData, batch storage.ReaderBatchWriter) error {
	latest, err := a.LatestIndexedHeight()
	if err != nil {
		return fmt.Errorf("failed to get latest indexed height: %w", err)
	}
	if data.Header.Height >= latest+1 {
		return ErrFutureHeight
	}
	if data.Header.Height <= latest {
		return ErrAlreadyIndexed
	}

	entries := make([]access.AccountTransaction, 0)
	chain := a.chainID.Chain()

	for i, tx := range data.Transactions {
		txIndex := uint32(i)
		addrMap := make(map[flow.Address]bool)
		addAddress := func(addr flow.Address, isAuthorizer bool) {
			if !chain.IsValid(addr) {
				return
			}
			if existing, ok := addrMap[addr]; ok {
				if !existing && isAuthorizer {
					addrMap[addr] = true
				}
				return
			}
			addrMap[addr] = isAuthorizer
		}

		addAddress(tx.Payer, false)
		addAddress(tx.ProposalKey.Address, false)
		for _, auth := range tx.Authorizers {
			addAddress(auth, true)
		}

		for _, event := range data.Events[txIndex] {
			info, err := a.transferParser.ParseTransferEvent(event)
			if err != nil {
				return fmt.Errorf("failed to extract addresses from event: %w", err)
			}
			if info == nil {
				continue
			}
			if info.From != nil {
				addAddress(*info.From, false)
			}
			if info.To != nil {
				addAddress(*info.To, false)
			}
		}

		for addr, isAuthorizer := range addrMap {
			entries = append(entries, access.AccountTransaction{
				Address:          addr,
				BlockHeight:      data.Header.Height,
				TransactionID:    tx.ID(),
				TransactionIndex: txIndex,
				IsAuthorizer:     isAuthorizer,
			})
		}
	}

	if err := a.store.Store(lctx, batch, data.Header.Height, entries); err != nil {
		return fmt.Errorf("failed to store account transactions: %w", err)
	}

	return nil
}
