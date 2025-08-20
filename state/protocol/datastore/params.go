package datastore

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

type Params struct {
	protocol.GlobalParams
	protocol.InstanceParams
}

var _ protocol.Params = (*Params)(nil)

// InstanceParams implements the interface [protocol.InstanceParams]. All values
// are cached after construction and do not incur database reads.
type InstanceParams struct {
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
func ReadInstanceParams(r storage.Reader, headers storage.Headers, seals storage.Seals) (*InstanceParams, error) {
	params := &InstanceParams{}

	// in next section we will read data from the database and cache them,
	// as they are immutable for the runtime of the node.
	var (
		finalizedRootHeight uint64
		sealedRootHeight    uint64
	)

	// root height
	err := operation.RetrieveRootHeight(r, &finalizedRootHeight)
	if err != nil {
		return nil, fmt.Errorf("could not read root block to populate cache: %w", err)
	}
	// sealed root height
	err = operation.RetrieveSealedRootHeight(r, &sealedRootHeight)
	if err != nil {
		return nil, fmt.Errorf("could not read sealed root block to populate cache: %w", err)
	}

	// look up 'finalized root block'
	var finalizedRootID flow.Identifier
	err = operation.LookupBlockHeight(r, finalizedRootHeight, &finalizedRootID)
	if err != nil {
		return nil, fmt.Errorf("could not look up finalized root height: %w", err)
	}
	params.finalizedRoot, err = headers.ByBlockID(finalizedRootID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve finalized root header: %w", err)
	}

	// look up the sealed block as of the 'finalized root block'
	var sealedRootID flow.Identifier
	err = operation.LookupBlockHeight(r, sealedRootHeight, &sealedRootID)
	if err != nil {
		return nil, fmt.Errorf("could not look up sealed root height: %w", err)
	}
	params.sealedRoot, err = headers.ByBlockID(sealedRootID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve sealed root header: %w", err)
	}

	// retrieve the root seal
	params.rootSeal, err = seals.HighestInFork(finalizedRootID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve root seal: %w", err)
	}

	return params, nil
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

// ReadFinalizedRoot retrieves the root block's header from the database.
// This information is immutable for the runtime of the software and may be cached.
func ReadFinalizedRoot(r storage.Reader) (*flow.Header, error) {
	var finalizedRootHeight uint64
	var rootID flow.Identifier
	var rootHeader flow.Header
	err := operation.RetrieveRootHeight(r, &finalizedRootHeight)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve finalized root height: %w", err)
	}
	err = operation.LookupBlockHeight(r, finalizedRootHeight, &rootID) // look up root block ID
	if err != nil {
		return nil, fmt.Errorf("could not retrieve root header's ID by height: %w", err)
	}
	err = operation.RetrieveHeader(r, rootID, &rootHeader) // retrieve root header
	if err != nil {
		return nil, fmt.Errorf("could not retrieve root header: %w", err)
	}

	return &rootHeader, nil
}
