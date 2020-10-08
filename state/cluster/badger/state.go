package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

type State struct {
	db        *badger.DB
	tracer    module.Tracer
	clusterID flow.ChainID
	headers   storage.Headers
	payloads  storage.ClusterPayloads
}

func NewState(db *badger.DB, tracer module.Tracer, clusterID flow.ChainID, headers storage.Headers, payloads storage.ClusterPayloads) (*State, error) {
	state := &State{
		db:        db,
		tracer:    tracer,
		clusterID: clusterID,
		headers:   headers,
		payloads:  payloads,
	}
	return state, nil
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

func (s *State) Mutate() cluster.Mutator {
	mutator := &Mutator{
		state: s,
	}
	return mutator
}
