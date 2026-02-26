package extended

import (
	"bytes"
	"context"
	"crypto/sha3"
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/state_synchronization/indexer/extended/events"
	"github.com/onflow/flow-go/storage"
)

const contractsIndexerName = "contracts"

const (
	flowAccountContractAdded   flow.EventType = "flow.AccountContractAdded"
	flowAccountContractUpdated flow.EventType = "flow.AccountContractUpdated"
	flowAccountContractRemoved flow.EventType = "flow.AccountContractRemoved"
)

// contractScriptExecutor is the subset of execution.ScriptExecutor used by the Contracts indexer.
type contractScriptExecutor interface {
	GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (*flow.Account, error)
}

// Contracts indexes contract deployment lifecycle events and writes to the contract deployments
// index. Handles [flow.AccountContractAdded] and [flow.AccountContractUpdated] events.
// [flow.AccountContractRemoved] is not currently supported; encountering one returns an error.
//
// Code retrieval via the script executor is best-effort. If code cannot be fetched (e.g. because
// the execution state is not yet synced for a block height), the deployment is stored without code
// bytes but with the code hash from the event, which is always authoritative.
//
// Not safe for concurrent use.
type Contracts struct {
	log            zerolog.Logger
	chain          flow.Chain
	store          storage.ContractDeploymentsIndexBootstrapper
	scriptExecutor contractScriptExecutor
}

var _ Indexer = (*Contracts)(nil)

// NewContracts creates a new Contracts indexer backed by store.
func NewContracts(
	log zerolog.Logger,
	chain flow.Chain,
	store storage.ContractDeploymentsIndexBootstrapper,
	scriptExecutor contractScriptExecutor,
) *Contracts {
	return &Contracts{
		log:            log.With().Str("component", "contracts_indexer").Logger(),
		chain:          chain,
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
// Expected error returns during normal operations:
//   - [ErrAlreadyIndexed]: if the data is already indexed for the height
func (c *Contracts) IndexBlockData(lctx lockctx.Proof, data BlockData, rw storage.ReaderBatchWriter) error {
	deployments, err := c.collectDeployments(data)
	if err != nil {
		return fmt.Errorf("failed to collect contract deployments for block %s: %w", data.Header.ID(), err)
	}
	if err := c.store.Store(lctx, rw, data.Header.Height, deployments); err != nil {
		if errors.Is(err, storage.ErrAlreadyExists) {
			return ErrAlreadyIndexed
		}
		return fmt.Errorf("failed to store contract deployments for block %s: %w", data.Header.ID(), err)
	}
	return nil
}

// collectDeployments iterates the block events and builds a [access.ContractDeployment] for
// each AccountContractAdded or AccountContractUpdated event found.
//
// Code retrieval is best-effort: failures are logged as warnings and the deployment is stored
// without code bytes (Code field is nil). The CodeHash from the event is always stored.
//
// No error returns are expected during normal operation.
func (c *Contracts) collectDeployments(data BlockData) ([]access.ContractDeployment, error) {
	retriever := newContractCodeRetriever(c.scriptExecutor, data.Header.Height)

	var deployments []access.ContractDeployment
	for _, event := range data.Events {
		switch event.Type {
		case flowAccountContractAdded:
			cadenceEvent, err := events.DecodePayload(event)
			if err != nil {
				return nil, fmt.Errorf("failed to decode %s event payload: %w", event.Type, err)
			}
			e, err := events.DecodeAccountContractAdded(cadenceEvent)
			if err != nil {
				return nil, fmt.Errorf("failed to decode %s event: %w", event.Type, err)
			}

			code := c.fetchCodeBestEffort(retriever, e.Address, e.ContractName, e.CodeHash, event.Type)

			deployments = append(deployments, access.ContractDeployment{
				ContractID:    events.ContractIDFromAddress(e.Address, e.ContractName),
				Address:       e.Address,
				BlockHeight:   data.Header.Height,
				TransactionID: event.TransactionID,
				TxIndex:       event.TransactionIndex,
				EventIndex:    event.EventIndex,
				Code:          code,
				CodeHash:      e.CodeHash,
			})

		case flowAccountContractUpdated:
			cadenceEvent, err := events.DecodePayload(event)
			if err != nil {
				return nil, fmt.Errorf("failed to decode %s event payload: %w", event.Type, err)
			}
			e, err := events.DecodeAccountContractUpdated(cadenceEvent)
			if err != nil {
				return nil, fmt.Errorf("failed to decode %s event: %w", event.Type, err)
			}

			code := c.fetchCodeBestEffort(retriever, e.Address, e.ContractName, e.CodeHash, event.Type)

			deployments = append(deployments, access.ContractDeployment{
				ContractID:    events.ContractIDFromAddress(e.Address, e.ContractName),
				Address:       e.Address,
				BlockHeight:   data.Header.Height,
				TransactionID: event.TransactionID,
				TxIndex:       event.TransactionIndex,
				EventIndex:    event.EventIndex,
				Code:          code,
				CodeHash:      e.CodeHash,
			})

		case flowAccountContractRemoved:
			// contract removal is not currently supported. returning an error here will cause the
			// indexer to crash, signalling that implementation is needed.
			return nil, fmt.Errorf("unexpected %s event in block %s tx %s: not supported",
				event.Type, data.Header.ID(), event.TransactionID)
		}
	}

	return deployments, nil
}

// fetchCodeBestEffort attempts to retrieve the contract code from the execution state. If
// retrieval fails for any reason (e.g. execution state not yet synced), a warning is logged
// and nil is returned so the deployment can be stored with just the code hash from the event.
func (c *Contracts) fetchCodeBestEffort(
	retriever *contractCodeRetriever,
	address flow.Address,
	contractName string,
	expectedHash []byte,
	eventType flow.EventType,
) []byte {
	if c.scriptExecutor == nil {
		return nil
	}

	code, err := retriever.contractCode(address, contractName)
	if err != nil {
		c.log.Warn().Err(err).
			Str("contract", events.ContractIDFromAddress(address, contractName)).
			Str("event", string(eventType)).
			Msg("could not retrieve contract code; storing deployment without code body")
		return nil
	}

	// Sanity check: verify the retrieved code matches the hash in the event.
	if !bytes.Equal(expectedHash, cadenceCodeToHash(code)) {
		c.log.Warn().
			Str("contract", events.ContractIDFromAddress(address, contractName)).
			Str("event", string(eventType)).
			Msg("contract code hash mismatch between execution state and event; storing deployment without code body")
		return nil
	}

	return code
}

// cadenceCodeToHash calculates the hash of the provided code using the same algorithm as cadence.
// This method should return the same value as the CodeHash field in flow.AccountContractAdded
// and flow.AccountContractUpdated events.
func cadenceCodeToHash(code []byte) []byte {
	// this is what cadence does in stdlib.CodeToHashValue()
	codeHash := sha3.Sum256(code)
	return codeHash[:]
}

// contractCodeRetriever lazily fetches and caches contract code per account address for a
// single block height.
//
// CAUTION: Not safe for concurrent use.
type contractCodeRetriever struct {
	scriptExecutor contractScriptExecutor
	height         uint64
	accounts       map[flow.Address]*flow.Account
}

func newContractCodeRetriever(scriptExecutor contractScriptExecutor, height uint64) *contractCodeRetriever {
	return &contractCodeRetriever{
		scriptExecutor: scriptExecutor,
		height:         height,
		accounts:       make(map[flow.Address]*flow.Account),
	}
}

// contractCode returns the code for the given contract on the given account, fetching the account
// state once per address and caching it for subsequent calls within the same block.
//
// CAUTION: Not safe for concurrent use.
//
// No error returns are expected during normal operation.
func (c *contractCodeRetriever) contractCode(address flow.Address, contractName string) ([]byte, error) {
	account, ok := c.accounts[address]
	if !ok {
		var err error
		account, err = c.scriptExecutor.GetAccountAtBlockHeight(context.TODO(), address, c.height)
		if err != nil {
			return nil, fmt.Errorf("failed to get account %s at height %d: %w", address.Hex(), c.height, err)
		}
		c.accounts[address] = account
	}

	code, ok := account.Contracts[contractName]
	if !ok {
		return nil, fmt.Errorf("contract %q not found on account %s at height %d", contractName, address.Hex(), c.height)
	}
	return code, nil
}
