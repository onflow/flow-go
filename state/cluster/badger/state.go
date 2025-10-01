package badger

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/consensus/hotstuff"
	clustermodel "github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/cluster/invalid"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/procedure"
)

type State struct {
	db        storage.DB
	clusterID flow.ChainID // the chain ID for the cluster
	epoch     uint64       // the operating epoch for the cluster
}

var _ cluster.State = (*State)(nil)

// Bootstrap initializes the persistent cluster state with a genesis block.
// The genesis block must have height 0, a parent hash of 32 zero bytes,
// and an empty collection as payload.
func Bootstrap(db storage.DB, lockManager lockctx.Manager, stateRoot *StateRoot) (*State, error) {
	lctx := lockManager.NewContext()
	defer lctx.Release()
	err := lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock `storage.LockInsertOrFinalizeClusterBlock` for inserting cluster block: %w", err)
	}
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
	err = state.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		chainID := genesis.ChainID
		// insert the block - by protocol convention, the genesis block does not have a proposer signature, which must be handled by the implementation
		proposal, err := clustermodel.NewRootProposal(
			clustermodel.UntrustedProposal{
				Block:           *genesis,
				ProposerSigData: nil,
			},
		)
		if err != nil {
			return fmt.Errorf("could not build root cluster proposal: %w", err)
		}
		err = procedure.InsertClusterBlock(lctx, rw, proposal)
		if err != nil {
			return fmt.Errorf("could not insert genesis block: %w", err)
		}
		// insert block height -> ID mapping
		err = operation.IndexClusterBlockHeight(lctx, rw.Writer(), chainID, genesis.Height, genesis.ID())
		if err != nil {
			return fmt.Errorf("failed to map genesis block height to block: %w", err)
		}
		// insert boundary
		err = operation.UpsertClusterFinalizedHeight(lctx, rw.Writer(), chainID, genesis.Height)
		// insert started view for hotstuff
		if err != nil {
			return fmt.Errorf("could not insert genesis boundary: %w", err)
		}

		safetyData := &hotstuff.SafetyData{
			LockedOneChainView:      genesis.View,
			HighestAcknowledgedView: genesis.View,
		}

		livenessData := &hotstuff.LivenessData{
			CurrentView: genesis.View + 1,
			NewestQC:    rootQC,
		}
		// insert safety data
		err = operation.UpsertSafetyData(rw.Writer(), chainID, safetyData)
		if err != nil {
			return fmt.Errorf("could not insert safety data: %w", err)
		}
		// insert liveness data
		err = operation.UpsertLivenessData(rw.Writer(), chainID, livenessData)
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

func OpenState(db storage.DB, _ module.Tracer, _ storage.Headers, _ storage.ClusterPayloads, clusterID flow.ChainID, epoch uint64) (*State, error) {
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

func newState(db storage.DB, clusterID flow.ChainID, epoch uint64) *State {
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
	// get height of latest finalized collection and then the ID of the collection with the corresponding height
	r := s.db.Reader()
	var latestFinalizedClusterHeight uint64
	err := operation.RetrieveClusterFinalizedHeight(r, s.clusterID, &latestFinalizedClusterHeight)
	if err != nil {
		return invalid.NewSnapshotf("could not retrieve finalized boundary: %w", err)
	}

	var blockID flow.Identifier
	err = operation.LookupClusterBlockHeight(r, s.clusterID, latestFinalizedClusterHeight, &blockID)
	if err != nil {
		return invalid.NewSnapshotf("could not retrieve finalized ID: %w", err)
	}

	return newSnapshot(s, blockID)
}

// AtBlockID returns the snapshot of the persistent cluster at the given
// block ID. It is available for any block that was introduced into the
// cluster state, and can thus represent an ambiguous state that was or
// will never be finalized.
// If the block is unknown, it returns an invalid snapshot, which returns
// state.ErrUnknownSnapshotReference for all methods
func (s *State) AtBlockID(blockID flow.Identifier) cluster.Snapshot {
	exists, err := operation.BlockExists(s.db.Reader(), blockID)
	if err != nil {
		return invalid.NewSnapshotf("could not check existence of reference block: %w", err)
	}
	if !exists {
		return invalid.NewSnapshotf("unknown block %x: %w", blockID, state.ErrUnknownSnapshotReference)
	}
	return newSnapshot(s, blockID)
}

// IsBootstrapped returns whether the database contains a bootstrapped state.
func IsBootstrapped(db storage.DB, clusterID flow.ChainID) (bool, error) {
	var finalized uint64
	err := operation.RetrieveClusterFinalizedHeight(db.Reader(), clusterID, &finalized)
	if errors.Is(err, storage.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("retrieving finalized height failed: %w", err)
	}
	return true, nil
}
