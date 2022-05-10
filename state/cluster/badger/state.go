package badger

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/procedure"
)

type State struct {
	db        *badger.DB
	clusterID flow.ChainID
}

// Bootstrap initializes the persistent cluster state with a genesis block.
// The genesis block must have height 0, a parent hash of 32 zero bytes,
// and an empty collection as payload.
func Bootstrap(db *badger.DB, stateRoot *StateRoot) (*State, error) {
	isBootstrapped, err := IsBootstrapped(db, stateRoot.ClusterID())
	if err != nil {
		return nil, fmt.Errorf("failed to determine whether database contains bootstrapped state: %w", err)
	}
	if isBootstrapped {
		return nil, fmt.Errorf("expected empty cluster state for cluster ID %s", stateRoot.ClusterID())
	}
	state := newState(db, stateRoot.ClusterID())

	genesis := stateRoot.Block()
	rootQC := stateRoot.QC()
	// bootstrap cluster state
	err = operation.RetryOnConflict(state.db.Update, func(tx *badger.Txn) error {
		chainID := genesis.Header.ChainID
		// insert the block
		err := procedure.InsertClusterBlock(genesis)(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis block: %w", err)
		}
		// insert block height -> ID mapping
		err = operation.IndexClusterBlockHeight(chainID, genesis.Header.Height, genesis.ID())(tx)
		if err != nil {
			return fmt.Errorf("failed to map genesis block height to block: %w", err)
		}
		// insert boundary
		err = operation.InsertClusterFinalizedHeight(chainID, genesis.Header.Height)(tx)
		// insert started view for hotstuff
		if err != nil {
			return fmt.Errorf("could not insert genesis boundary: %w", err)
		}

		safetyData := &hotstuff.SafetyData{
			LockedOneChainView:      genesis.Header.View,
			HighestAcknowledgedView: genesis.Header.View,
		}

		livenessData := &hotstuff.LivenessData{
			CurrentView: genesis.Header.View + 1,
			HighestQC:   rootQC,
		}
		// insert safety data
		err = operation.InsertSafetyData(chainID, safetyData)(tx)
		if err != nil {
			return fmt.Errorf("could not insert safety data: %w", err)
		}
		// insert liveness data
		err = operation.InsertLivenessData(chainID, livenessData)(tx)
		if err != nil {
			return fmt.Errorf("could not insert liveness data: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("bootstrapping failed: %w", err)
	}

	return state, nil
}

func OpenState(db *badger.DB, tracer module.Tracer, headers storage.Headers, payloads storage.ClusterPayloads, clusterID flow.ChainID) (*State, error) {
	isBootstrapped, err := IsBootstrapped(db, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to determine whether database contains bootstrapped state: %w", err)
	}
	if !isBootstrapped {
		return nil, fmt.Errorf("expected database to contain bootstrapped state")
	}
	state := newState(db, clusterID)
	return state, nil
}

func newState(db *badger.DB, clusterID flow.ChainID) *State {
	state := &State{
		db:        db,
		clusterID: clusterID,
	}
	return state
}

func (s *State) Params() cluster.Params {
	params := &Params{
		state: s,
	}
	return params
}

func (s *State) Final() cluster.Snapshot {
	// get the finalized block ID
	var blockID flow.Identifier
	err := s.db.View(func(tx *badger.Txn) error {
		var boundary uint64
		err := operation.RetrieveClusterFinalizedHeight(s.clusterID, &boundary)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve finalized boundary: %w", err)
		}

		err = operation.LookupClusterBlockHeight(s.clusterID, boundary, &blockID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve finalized ID: %w", err)
		}

		return nil
	})
	if err != nil {
		return &Snapshot{
			err: err,
		}
	}

	snapshot := &Snapshot{
		state:   s,
		blockID: blockID,
	}
	return snapshot
}

func (s *State) AtBlockID(blockID flow.Identifier) cluster.Snapshot {
	snapshot := &Snapshot{
		state:   s,
		blockID: blockID,
	}
	return snapshot
}

// IsBootstrapped returns whether or not the database contains a bootstrapped state
func IsBootstrapped(db *badger.DB, clusterID flow.ChainID) (bool, error) {
	var finalized uint64
	err := db.View(operation.RetrieveClusterFinalizedHeight(clusterID, &finalized))
	if errors.Is(err, storage.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("retrieving finalized height failed: %w", err)
	}
	return true, nil
}
