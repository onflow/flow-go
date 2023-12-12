package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

type Params struct {
	protocol.GlobalParams
	protocol.InstanceParams
}

var _ protocol.Params = (*Params)(nil)

// InstanceParams implements the interface protocol.InstanceParams. All functions
// are served on demand directly from the database, _without_ any caching.
type InstanceParams struct {
	db *badger.DB
	// finalizedRoot marks the cutoff of the history this node knows about. It is the block at the tip
	// of the root snapshot used to bootstrap this node - all newer blocks are synced from the network.
	finalizedRoot *flow.Header
	// sealedRoot is the latest sealed block with respect to `finalizedRoot`.
	sealedRoot *flow.Header
	// rootSeal is the seal for block `sealedRoot` - the newest incorporated seal with respect to `finalizedRoot`.
	rootSeal *flow.Seal
}

var _ protocol.InstanceParams = (*InstanceParams)(nil)

// ReadInstanceParams reads the instance parameters from the database and returns them as in-memory representation.
// No errors are expected during normal operation.
func ReadInstanceParams(db *badger.DB, headers storage.Headers, seals storage.Seals) (*InstanceParams, error) {
	params := &InstanceParams{
		db: db,
	}

	// in next section we will read data from the database and cache them,
	// as they are immutable for the runtime of the node.
	err := db.View(func(txn *badger.Txn) error {
		var (
			finalizedRootHeight uint64
			sealedRootHeight    uint64
		)

		// root height
		err := db.View(operation.RetrieveRootHeight(&finalizedRootHeight))
		if err != nil {
			return fmt.Errorf("could not read root block to populate cache: %w", err)
		}
		// sealed root height
		err = db.View(operation.RetrieveSealedRootHeight(&sealedRootHeight))
		if err != nil {
			return fmt.Errorf("could not read sealed root block to populate cache: %w", err)
		}

		// look up 'finalized root block'
		var finalizedRootID flow.Identifier
		err = db.View(operation.LookupBlockHeight(finalizedRootHeight, &finalizedRootID))
		if err != nil {
			return fmt.Errorf("could not look up finalized root height: %w", err)
		}
		params.finalizedRoot, err = headers.ByBlockID(finalizedRootID)
		if err != nil {
			return fmt.Errorf("could not retrieve finalized root header: %w", err)
		}

		// look up the sealed block as of the 'finalized root block'
		var sealedRootID flow.Identifier
		err = db.View(operation.LookupBlockHeight(sealedRootHeight, &sealedRootID))
		if err != nil {
			return fmt.Errorf("could not look up sealed root height: %w", err)
		}
		params.sealedRoot, err = headers.ByBlockID(sealedRootID)
		if err != nil {
			return fmt.Errorf("could not retrieve sealed root header: %w", err)
		}

		// retrieve the root seal
		params.rootSeal, err = seals.HighestInFork(finalizedRootID)
		if err != nil {
			return fmt.Errorf("could not retrieve root seal: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not read InstanceParams data to populate cache: %w", err)
	}

	return params, nil
}

// EpochFallbackTriggered returns whether epoch fallback mode [EFM] has been triggered.
// EFM is a permanent, spork-scoped state which is triggered when the next
// epoch fails to be committed in the allocated time. Once EFM is triggered,
// it will remain in effect until the next spork.
// TODO for 'leaving Epoch Fallback via special service event'
// No errors are expected during normal operation.
func (p *InstanceParams) EpochFallbackTriggered() (bool, error) {
	var triggered bool
	err := p.db.View(operation.CheckEpochEmergencyFallbackTriggered(&triggered))
	if err != nil {
		return false, fmt.Errorf("could not check epoch fallback triggered: %w", err)
	}
	return triggered, nil
}

// FinalizedRoot returns the finalized root header of the current protocol state. This will be
// the head of the protocol state snapshot used to bootstrap this state and
// may differ from node to node for the same protocol state.
func (p *InstanceParams) FinalizedRoot() *flow.Header {
	return p.finalizedRoot
}

// SealedRoot returns the sealed root block. If it's different from FinalizedRoot() block,
// it means the node is bootstrapped from mid-spork.
func (p *InstanceParams) SealedRoot() *flow.Header {
	return p.sealedRoot
}

// Seal returns the root block seal of the current protocol state. This is the seal for the
// `SealedRoot` block that was used to bootstrap this state. It may differ from node to node.
func (p *InstanceParams) Seal() *flow.Seal {
	return p.rootSeal
}

// ReadGlobalParams reads the global parameters from the database and returns them as in-memory representation.
// No errors are expected during normal operation.
func ReadGlobalParams(db *badger.DB) (*inmem.Params, error) {
	var sporkID flow.Identifier
	err := db.View(operation.RetrieveSporkID(&sporkID))
	if err != nil {
		return nil, fmt.Errorf("could not get spork id: %w", err)
	}

	var sporkRootBlockHeight uint64
	err = db.View(operation.RetrieveSporkRootBlockHeight(&sporkRootBlockHeight))
	if err != nil {
		return nil, fmt.Errorf("could not get spork root block height: %w", err)
	}

	var threshold uint64
	err = db.View(operation.RetrieveEpochCommitSafetyThreshold(&threshold))
	if err != nil {
		return nil, fmt.Errorf("could not get epoch commit safety threshold")
	}

	var version uint
	err = db.View(operation.RetrieveProtocolVersion(&version))
	if err != nil {
		return nil, fmt.Errorf("could not get protocol version: %w", err)
	}

	root, err := ReadFinalizedRoot(db) // retrieve root header
	if err != nil {
		return nil, fmt.Errorf("could not get root: %w", err)
	}

	return inmem.NewParams(
		inmem.EncodableParams{
			ChainID:                    root.ChainID,
			SporkID:                    sporkID,
			SporkRootBlockHeight:       sporkRootBlockHeight,
			ProtocolVersion:            version,
			EpochCommitSafetyThreshold: threshold,
		},
	), nil
}

// ReadFinalizedRoot retrieves the root block's header from the database.
// This information is immutable for the runtime of the software and may be cached.
func ReadFinalizedRoot(db *badger.DB) (*flow.Header, error) {
	var finalizedRootHeight uint64
	var rootID flow.Identifier
	var rootHeader flow.Header
	err := db.View(func(tx *badger.Txn) error {
		err := operation.RetrieveRootHeight(&finalizedRootHeight)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve finalized root height: %w", err)
		}
		err = operation.LookupBlockHeight(finalizedRootHeight, &rootID)(tx) // look up root block ID
		if err != nil {
			return fmt.Errorf("could not retrieve root header's ID by height: %w", err)
		}
		err = operation.RetrieveHeader(rootID, &rootHeader)(tx) // retrieve root header
		if err != nil {
			return fmt.Errorf("could not retrieve root header: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to read root information from database: %w", err)
	}
	return &rootHeader, nil
}
