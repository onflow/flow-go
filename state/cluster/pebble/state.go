package pebble

import (
	"errors"
	"fmt"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/pebble/operation"
	"github.com/onflow/flow-go/storage/pebble/procedure"
)

type State struct {
	db        *pebble.DB
	clusterID flow.ChainID // the chain ID for the cluster
	epoch     uint64       // the operating epoch for the cluster
}

// Bootstrap initializes the persistent cluster state with a genesis block.
// The genesis block must have height 0, a parent hash of 32 zero bytes,
// and an empty collection as payload.
func Bootstrap(db *pebble.DB, stateRoot *StateRoot) (*State, error) {
	isBootstrapped, err := IsBootstrapped(db, stateRoot.ClusterID())
	if err != nil {
		return nil, fmt.Errorf("failed to determine whether database contains bootstrapped state: %w", err)
	}
	if isBootstrapped {
		return nil, fmt.Errorf("expected empty cluster state for cluster ID %s", stateRoot.ClusterID())
	}
	state := newState(db, stateRoot.ClusterID(), stateRoot.EpochCounter())

	genesis := stateRoot.Block()
	rootQC := stateRoot.QC()
	// bootstrap cluster state
	err = operation.WithReaderBatchWriter(state.db, func(tx storage.PebbleReaderBatchWriter) error {
		_, w := tx.ReaderWriter()
		chainID := genesis.Header.ChainID
		// insert the block
		err := procedure.InsertClusterBlock(genesis)(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis block: %w", err)
		}
		// insert block height -> ID mapping
		err = operation.IndexClusterBlockHeight(chainID, genesis.Header.Height, genesis.ID())(w)
		if err != nil {
			return fmt.Errorf("failed to map genesis block height to block: %w", err)
		}
		// insert boundary
		err = operation.InsertClusterFinalizedHeight(chainID, genesis.Header.Height)(w)
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
			NewestQC:    rootQC,
		}
		// insert safety data
		err = operation.InsertSafetyData(chainID, safetyData)(w)
		if err != nil {
			return fmt.Errorf("could not insert safety data: %w", err)
		}
		// insert liveness data
		err = operation.InsertLivenessData(chainID, livenessData)(w)
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

func OpenState(db *pebble.DB, _ module.Tracer, _ storage.Headers, _ storage.ClusterPayloads, clusterID flow.ChainID, epoch uint64) (*State, error) {
	isBootstrapped, err := IsBootstrapped(db, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to determine whether database contains bootstrapped state: %w", err)
	}
	if !isBootstrapped {
		return nil, fmt.Errorf("expected database to contain bootstrapped state")
	}
	state := newState(db, clusterID, epoch)
	return state, nil
}

func newState(db *pebble.DB, clusterID flow.ChainID, epoch uint64) *State {
	state := &State{
		db:        db,
		clusterID: clusterID,
		epoch:     epoch,
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
	err := (func(tx pebble.Reader) error {
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
	})(s.db)
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

// IsBootstrapped returns whether the database contains a bootstrapped state.
func IsBootstrapped(db *pebble.DB, clusterID flow.ChainID) (bool, error) {
	var finalized uint64
	err := operation.RetrieveClusterFinalizedHeight(clusterID, &finalized)(db)
	if errors.Is(err, storage.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("retrieving finalized height failed: %w", err)
	}
	return true, nil
}
