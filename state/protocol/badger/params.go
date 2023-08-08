package badger

import (
	"fmt"
	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/flow-go/storage"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/storage/badger/operation"
)

type Params struct {
	protocol.GlobalParams
	state *State
}

var _ protocol.InstanceParams = (*Params)(nil)
var _ protocol.GlobalParams = (*Params)(nil) // TODO(yuraolex): probably this is temporary since protocol state will be serving global params

func (p Params) EpochFallbackTriggered() (bool, error) {
	var triggered bool
	err := p.state.db.View(operation.CheckEpochEmergencyFallbackTriggered(&triggered))
	if err != nil {
		return false, fmt.Errorf("could not check epoch fallback triggered: %w", err)
	}
	return triggered, nil
}

func (p Params) FinalizedRoot() (*flow.Header, error) {

	// look up root block ID
	var rootID flow.Identifier
	err := p.state.db.View(operation.LookupBlockHeight(p.state.finalizedRootHeight, &rootID))
	if err != nil {
		return nil, fmt.Errorf("could not look up root header: %w", err)
	}

	// retrieve root header
	header, err := p.state.headers.ByBlockID(rootID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve root header: %w", err)
	}

	return header, nil
}

func (p Params) SealedRoot() (*flow.Header, error) {
	// look up root block ID
	var rootID flow.Identifier
	err := p.state.db.View(operation.LookupBlockHeight(p.state.sealedRootHeight, &rootID))

	if err != nil {
		return nil, fmt.Errorf("could not look up root header: %w", err)
	}

	// retrieve root header
	header, err := p.state.headers.ByBlockID(rootID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve root header: %w", err)
	}

	return header, nil
}

func (p Params) Seal() (*flow.Seal, error) {

	// look up root header
	var rootID flow.Identifier
	err := p.state.db.View(operation.LookupBlockHeight(p.state.finalizedRootHeight, &rootID))
	if err != nil {
		return nil, fmt.Errorf("could not look up root header: %w", err)
	}

	// retrieve the root seal
	seal, err := p.state.seals.HighestInFork(rootID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve root seal: %w", err)
	}

	return seal, nil
}

// ReadGlobalParams reads the global parameters from the database and returns them as in-memory representation.
// No errors are expected during normal operation.
func ReadGlobalParams(db *badger.DB, headers storage.Headers) (*inmem.Params, error) {
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

	root, err := ReadFinalizedRoot(db, headers)
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

func ReadFinalizedRoot(db *badger.DB, headers storage.Headers) (*flow.Header, error) {
	var finalizedRootHeight uint64
	err := db.View(operation.RetrieveRootHeight(&finalizedRootHeight))

	// look up root block ID
	var rootID flow.Identifier
	err = db.View(operation.LookupBlockHeight(finalizedRootHeight, &rootID))
	if err != nil {
		return nil, fmt.Errorf("could not look up root header: %w", err)
	}

	// retrieve root header
	header, err := headers.ByBlockID(rootID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve root header: %w", err)
	}

	return header, nil
}
