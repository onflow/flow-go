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

	// rootHeight marks the cutoff of the history this node knows about. We cache it in the state
	// because it cannot change over the lifecycle of a protocol state instance. It is frequently
	// larger than the height of the root block of the spork, (also cached below as
	// `sporkRootBlockHeight`), for instance, if the node joined in an epoch after the last spork.
	finalizedRoot *flow.Header
	// sealedRootHeight returns the root block that is sealed.
	sealedRoot *flow.Header
	// sporkRootBlockHeight is the height of the root block in the current spork. We cache it in
	// the state, because it cannot change over the lifecycle of a protocol state instance.
	// Caution: A node that joined in a later epoch past the spork, the node will likely _not_
	// know the spork's root block in full (though it will always know the height).
	sporkRootBlockHeight uint64
	// rootSeal stores the root block seal of the current protocol state.
	rootSeal *flow.Seal
}

var _ protocol.InstanceParams = (*InstanceParams)(nil)

func NewInstanceParams(db *badger.DB, headers storage.Headers, seals storage.Seals) (*InstanceParams, error) {
	params := &InstanceParams{
		db: db,
	}

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
		// spork root block height
		err = db.View(operation.RetrieveSporkRootBlockHeight(&params.sporkRootBlockHeight))
		if err != nil {
			return fmt.Errorf("could not get spork root block height: %w", err)
		}

		// look up root block ID
		var finalizedRootID flow.Identifier
		err = db.View(operation.LookupBlockHeight(finalizedRootHeight, &finalizedRootID))
		if err != nil {
			return fmt.Errorf("could not look up finalized root height: %w", err)
		}

		params.finalizedRoot, err = headers.ByBlockID(finalizedRootID)
		if err != nil {
			return fmt.Errorf("could not retrieve finalized root header: %w", err)
		}

		var sealedRootID flow.Identifier
		err = db.View(operation.LookupBlockHeight(sealedRootHeight, &sealedRootID))
		if err != nil {
			return fmt.Errorf("could not look up sealed root height: %w", err)
		}

		// retrieve root header
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
		return nil, fmt.Errorf("could not read root data to populate cache: %w", err)
	}

	return params, nil
}

func (p *InstanceParams) EpochFallbackTriggered() (bool, error) {
	var triggered bool
	err := p.db.View(operation.CheckEpochEmergencyFallbackTriggered(&triggered))
	if err != nil {
		return false, fmt.Errorf("could not check epoch fallback triggered: %w", err)
	}
	return triggered, nil
}

func (p *InstanceParams) FinalizedRoot() (*flow.Header, error) {
	return p.finalizedRoot, nil
}

func (p *InstanceParams) SealedRoot() (*flow.Header, error) {
	return p.sealedRoot, nil
}

// Seal returns the root block seal of the current protocol state. This will be
// the seal for the root block used to bootstrap this state and may differ from
// node to node for the same protocol state.
// No errors are expected during normal operation.
func (p *InstanceParams) Seal() (*flow.Seal, error) {
	return p.rootSeal, nil
}

// SporkRootBlockHeight is the height of the root block in the current spork.
func (p *InstanceParams) SporkRootBlockHeight() uint64 {
	return p.sporkRootBlockHeight
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

	// retrieve root header

	root, err := ReadFinalizedRoot(db)
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
