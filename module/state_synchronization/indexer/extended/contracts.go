package extended

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/state_synchronization/indexer/extended/events"
	"github.com/onflow/flow-go/storage"
)

const (
	contractsIndexerName = "contracts"

	flowAccountContractAdded   flow.EventType = "flow.AccountContractAdded"
	flowAccountContractUpdated flow.EventType = "flow.AccountContractUpdated"
	flowAccountContractRemoved flow.EventType = "flow.AccountContractRemoved"
)

// snapshotProvider is the subset of execution.ScriptExecutor used by the Contracts indexer.
type snapshotProvider interface {
	GetStorageSnapshot(height uint64) (snapshot.StorageSnapshot, error)
}

// Contracts indexes contract deployment lifecycle events and writes to the contract deployments
// index. Handles [flow.AccountContractAdded] and [flow.AccountContractUpdated] events.
// [flow.AccountContractRemoved] is not currently permitted; encountering one returns an error.
//
// On first bootstrapping, the indexer will load all deployed contracts from storage at the bootstrap
// height and include placeholder deployment objects for all contracts that are deployed at that height.
// Any contracts that are deployed in the same block as the bootstrap height will not have placeholder
// deployment objects created since there are already deployment objects for those contracts.
//
// CAUTION: Not safe for concurrent use.
type Contracts struct {
	log            zerolog.Logger
	chain          flow.Chain
	metrics        module.ExtendedIndexingMetrics
	store          storage.ContractDeploymentsIndexBootstrapper
	scriptExecutor snapshotProvider
}

var _ Indexer = (*Contracts)(nil)

// NewContracts creates a new Contracts indexer backed by store.
func NewContracts(
	log zerolog.Logger,
	chain flow.Chain,
	store storage.ContractDeploymentsIndexBootstrapper,
	scriptExecutor snapshotProvider,
	metrics module.ExtendedIndexingMetrics,
) *Contracts {
	return &Contracts{
		log:            log.With().Str("component", "contracts_indexer").Logger(),
		chain:          chain,
		metrics:        metrics,
		store:          store,
		scriptExecutor: scriptExecutor,
	}
}

// Name returns the name of this indexer.
func (c *Contracts) Name() string { return contractsIndexerName }

// NextHeight returns the next block height to index.
//
// No error returns are expected during normal operation.
func (c *Contracts) NextHeight() (uint64, error) {
	return nextHeight(c.store)
}

// IndexBlockData processes one block's events and transactions and updates the contract
// deployments index.
//
// The caller must hold the [storage.LockIndexContractDeployments] lock until the batch is committed.
//
// CAUTION: Not safe for concurrent use.
//
// Expected error returns during normal operations:
//   - [ErrAlreadyIndexed]: if the data is already indexed for the height
func (c *Contracts) IndexBlockData(lctx lockctx.Proof, data BlockData, rw storage.ReaderBatchWriter) error {
	deployments, created, updated, err := c.collectDeployments(data)
	if err != nil {
		return fmt.Errorf("failed to collect contract deployments for block %s: %w", data.Header.ID(), err)
	}

	// if storage is not bootstrapped yet, do an initial load of all deployed contracts and include it in
	// the initial store operation.
	// TODO: avoid doing this check on every block. in the mean time, this is a fast check
	if bootstrapHeight, isInitialized := c.store.UninitializedFirstHeight(); !isInitialized {
		start := time.Now()

		// keep track of the contracts with deployments in this block. these can be skipped during the
		// initial load since the deployment object is already created.
		updatedContracts := make(map[string]bool, len(deployments))
		for _, deployment := range deployments {
			updatedContracts[deployment.ContractID] = true
		}

		bootstrapContracts, err := c.loadDeployedContracts(bootstrapHeight, updatedContracts)
		if err != nil {
			return fmt.Errorf("failed to load deployed contracts: %w", err)
		}
		deployments = append(deployments, bootstrapContracts...)

		c.log.Info().
			Uint64("height", bootstrapHeight).
			Int("contracts", len(bootstrapContracts)).
			Dur("dur_ms", time.Since(start)).
			Msg("loaded deployed contracts during bootstrap")
	}

	if err := c.store.Store(lctx, rw, data.Header.Height, deployments); err != nil {
		if errors.Is(err, storage.ErrAlreadyExists) {
			return ErrAlreadyIndexed
		}
		return fmt.Errorf("failed to store contract deployments for block %s: %w", data.Header.ID(), err)
	}

	c.metrics.ContractDeploymentIndexed(created, updated) // ignores bootstrap deployments
	return nil
}

// collectDeployments iterates the block events and builds a [access.ContractDeployment] for
// each AccountContractAdded or AccountContractUpdated event found. Returns the deployments
// and the counts of created and updated contracts.
//
// No error returns are expected during normal operation.
func (c *Contracts) collectDeployments(data BlockData) (deployments []access.ContractDeployment, created, updated int, err error) {
	retriever := newContractRetriever(c.scriptExecutor, data.Header.Height)

	for _, event := range data.Events {
		switch event.Type {
		case flowAccountContractAdded:
			cadenceEvent, err := events.DecodePayload(event)
			if err != nil {
				return nil, 0, 0, fmt.Errorf("failed to decode %s event payload: %w", event.Type, err)
			}
			e, err := events.DecodeAccountContractAdded(cadenceEvent)
			if err != nil {
				return nil, 0, 0, fmt.Errorf("failed to decode %s event: %w", event.Type, err)
			}

			// script executor gets data at the end of the block, so code here should be the updated version
			code, err := retriever.contractCode(e.Address, e.ContractName, data.Header.Height)
			if err != nil {
				return nil, 0, 0, fmt.Errorf("failed to get contract code: %w", err)
			}

			// make sure the hash of the code fetched from state matches the hash in the event
			if !bytes.Equal(e.CodeHash, access.CadenceCodeHash(code)) {
				return nil, 0, 0, fmt.Errorf("code hash mismatch for %s event: %s", event.Type, e.ContractName)
			}

			deployments = append(deployments, access.ContractDeployment{
				ContractID:       events.ContractIDFromAddress(e.Address, e.ContractName),
				Address:          e.Address,
				BlockHeight:      data.Header.Height,
				TransactionID:    event.TransactionID,
				TransactionIndex: event.TransactionIndex,
				EventIndex:       event.EventIndex,
				Code:             code,
				CodeHash:         e.CodeHash,
			})
			created++

		case flowAccountContractUpdated:
			cadenceEvent, err := events.DecodePayload(event)
			if err != nil {
				return nil, 0, 0, fmt.Errorf("failed to decode %s event payload: %w", event.Type, err)
			}
			e, err := events.DecodeAccountContractUpdated(cadenceEvent)
			if err != nil {
				return nil, 0, 0, fmt.Errorf("failed to decode %s event: %w", event.Type, err)
			}

			// script executor gets data at the end of the block, so code here should be the updated version
			code, err := retriever.contractCode(e.Address, e.ContractName, data.Header.Height)
			if err != nil {
				return nil, 0, 0, fmt.Errorf("failed to get account code: %w", err)
			}

			// make sure the hash of the code fetched from state matches the hash in the event
			if !bytes.Equal(e.CodeHash, access.CadenceCodeHash(code)) {
				return nil, 0, 0, fmt.Errorf("code hash mismatch for %s event: %s", event.Type, e.ContractName)
			}

			deployments = append(deployments, access.ContractDeployment{
				ContractID:       events.ContractIDFromAddress(e.Address, e.ContractName),
				Address:          e.Address,
				BlockHeight:      data.Header.Height,
				TransactionID:    event.TransactionID,
				TransactionIndex: event.TransactionIndex,
				EventIndex:       event.EventIndex,
				Code:             code,
				CodeHash:         e.CodeHash,
			})
			updated++

		case flowAccountContractRemoved:
			// contract removal is not currently supported. returning an error here will cause the
			// indexer to crash, signalling that implementation is needed.
			return nil, 0, 0, fmt.Errorf("unexpected %s event in block %s tx %s: not supported",
				event.Type, data.Header.ID(), event.TransactionID)
		}
	}

	return deployments, created, updated, nil
}

// loadDeployedContracts loads all deployed contracts from storage at the given height and returns a
// list of access.ContractDeployment records.
//
// No error returns are expected during normal operation.
func (c *Contracts) loadDeployedContracts(height uint64, seenContracts map[string]bool) ([]access.ContractDeployment, error) {
	snapshot, err := c.scriptExecutor.GetStorageSnapshot(height)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage snapshot: %w", err)
	}

	txnState := state.NewTransactionState(snapshot, state.DefaultParameters())
	accounts := environment.NewAccounts(txnState)

	generator := c.chain.NewAddressGenerator()

	var deployments []access.ContractDeployment

	for {
		address, err := generator.NextAddress()
		if err != nil {
			return nil, fmt.Errorf("cannot get address: %w", err)
		}

		exists, err := accounts.Exists(address)
		if err != nil {
			return nil, fmt.Errorf("error while checking if account exists: %w", err)
		}

		// iterate until we find the first account that does not exist in the snapshot.
		if !exists {
			return deployments, nil
		}

		contractNames, err := accounts.GetContractNames(address)
		if err != nil {
			return nil, fmt.Errorf("error while getting contract names: %w", err)
		}

		for _, contractName := range contractNames {
			contractID := events.ContractIDFromAddress(address, contractName)

			// skip any contracts that were already updated in this block since the deployment object
			// is already created
			if seenContracts[contractID] {
				continue
			}

			code, err := accounts.GetContract(contractName, address)
			if err != nil {
				return nil, fmt.Errorf("error while getting contract: %w", err)
			}

			deployments = append(deployments, access.ContractDeployment{
				ContractID: contractID,
				Address:    address,
				Code:       code,
				CodeHash:   access.CadenceCodeHash(code),
				// all other fields are omitted because we do not know the actual deployment details
				IsPlaceholder: true,
			})
		}
	}
}

// contractRetriever is a helper for retrieving contract code from the snapshot at the given height.
// It lazily sets up the accounts instance, so unused instances are cheap to create.
//
// CAUTION: Not safe for concurrent use.
type contractRetriever struct {
	height         uint64
	accounts       environment.Accounts
	scriptExecutor snapshotProvider
}

func newContractRetriever(scriptExecutor snapshotProvider, height uint64) *contractRetriever {
	return &contractRetriever{
		height:         height,
		scriptExecutor: scriptExecutor,
	}
}

// contractCode returns the code for the given contract at the given height.
//
// CAUTION: Not safe for concurrent use.
//
// No error returns are expected during normal operation.
func (c *contractRetriever) contractCode(
	address flow.Address,
	contractName string,
	height uint64,
) ([]byte, error) {
	if c.height != height {
		return nil, fmt.Errorf("height mismatch: %d != %d", c.height, height)
	}

	// lazily setup accounts since many blocks will not have any contract updates.
	if c.accounts == nil {
		snapshot, err := c.scriptExecutor.GetStorageSnapshot(height)
		if err != nil {
			return nil, fmt.Errorf("failed to get storage snapshot: %w", err)
		}

		txnState := state.NewTransactionState(snapshot, state.DefaultParameters())
		c.accounts = environment.NewAccounts(txnState)
	}

	code, err := c.accounts.GetContract(contractName, address)
	if err != nil {
		return nil, fmt.Errorf("error while getting contract: %w", err)
	}

	return code, nil
}
