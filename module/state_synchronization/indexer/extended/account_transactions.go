package extended

import (
	"errors"
	"fmt"
	"slices"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/access/systemcollection"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

const accountTransactionsIndexerName = "account_transactions"

// AccountTransactions indexes account-transaction associations for a block.
type AccountTransactions struct {
	log                      zerolog.Logger
	store                    storage.AccountTransactionsBootstrapper
	chainID                  flow.ChainID
	lockManager              storage.LockManager
	serviceAccount           flow.Address
	scheduledExecutorAccount flow.Address
	systemCollections        *systemcollection.Versioned
}

var _ Indexer = (*AccountTransactions)(nil)

func NewAccountTransactions(
	log zerolog.Logger,
	store storage.AccountTransactionsBootstrapper,
	chainID flow.ChainID,
	lockManager storage.LockManager,
) (*AccountTransactions, error) {
	sc := systemcontracts.SystemContractsForChain(chainID)
	systemCollections, err := systemcollection.NewVersioned(chainID.Chain(), systemcollection.Default(chainID))
	if err != nil {
		return nil, fmt.Errorf("failed to create system collection set: %w", err)
	}

	return &AccountTransactions{
		log:                      log.With().Str("component", "account_tx_indexer").Logger(),
		store:                    store,
		chainID:                  chainID,
		lockManager:              lockManager,
		serviceAccount:           sc.FlowServiceAccount.Address,
		scheduledExecutorAccount: sc.ScheduledTransactionExecutor.Address,
		systemCollections:        systemCollections,
	}, nil
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

	addRole := func(addrRoles map[flow.Address][]access.TransactionRole, addr flow.Address, role access.TransactionRole) {
		if _, ok := addrRoles[addr]; !ok {
			addrRoles[addr] = make([]access.TransactionRole, 0)
		}
		addrRoles[addr] = append(addrRoles[addr], role)
	}

	// By the Flow protocol, system chunk transactions always appear after all user transactions
	// in a block. Once the first system transaction is encountered, all subsequent transactions
	// are also part of the system chunk.
	isSystemChunk := false
	for i, tx := range data.Transactions {
		txIndex := uint32(i)
		txID := tx.ID()
		_, isSystemTx := a.systemCollections.SearchAll(txID)
		// all tx after the first system tx are in the system chunk
		isSystemChunk = isSystemChunk || isSystemTx

		// Track roles per address. An address can have multiple roles (e.g., payer AND authorizer).
		addrRoles := make(map[flow.Address][]access.TransactionRole)

		addRole(addrRoles, tx.Payer, access.TransactionRolePayer)
		addRole(addrRoles, tx.ProposalKey.Address, access.TransactionRoleProposer)
		for _, auth := range tx.Authorizers {
			// the service account authorizes all system transactions, and the scheduled tx executor
			// account authorizes all scheduled transactions. skip indexing since we can derive them
			// as needed.
			if isSystemTx || isSystemChunk {
				if auth == a.serviceAccount || auth == a.scheduledExecutorAccount {
					continue
				}
			}
			addRole(addrRoles, auth, access.TransactionRoleAuthorizer)
		}

		seen := make(map[flow.Address]struct{})
		for _, event := range data.Events[txIndex] {
			eventAddresses, err := a.extractAddresses(event)
			if err != nil {
				return nil, fmt.Errorf("failed to extract addresses from event: %w", err)
			}
			for _, addr := range eventAddresses {
				// only add the role once for an address per transaction
				if _, ok := seen[addr]; ok {
					continue
				}
				seen[addr] = struct{}{}

				// since `extractAddresses` returns all [cadence.Address] fields in the event, it's possible
				// that the event contains invalid addresses, or addresses from a different chain.
				// Only index addresses that are actually valid for the current chain.
				if chain.IsValid(addr) {
					addRole(addrRoles, addr, access.TransactionRoleInteracted)
				}
			}
		}

		for addr, roles := range addrRoles {
			slices.Sort(roles) // sort roles in ascending order
			entries = append(entries, access.AccountTransaction{
				Address:          addr,
				BlockHeight:      data.Header.Height,
				TransactionID:    txID,
				TransactionIndex: txIndex,
				Roles:            roles,
			})
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
