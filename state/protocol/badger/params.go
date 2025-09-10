package badger

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
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
// are cached after construction and do not incur database reads. The values are
// constant throughout the lifetime of a node; therefore, non-atomic reads of the
// fields via accessor methods are acceptable.
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
// It serves as a constructor for InstanceParams and only requires a read-only database handle,
// emphasizing that it only reads and never writes.
// No errors are expected during normal operation.
func ReadInstanceParams(r storage.Reader, headers storage.Headers, seals storage.Seals) (*InstanceParams, error) {
	params := &InstanceParams{}

	// The values below are written during bootstrapping and immutable for the lifetime of the node. All
	// following parameters are uniquely defined by the values initially read.
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

// ReadSporkRootBlock reads the spork root block from the database.
// No errors are expected during normal operation.
func ReadSporkRootBlock(
	db storage.DB,
	blocks storage.Blocks,
) (*flow.Block, error) {
	var sporkRootBlockID flow.Identifier
	err := operation.RetrieveSporkRootBlockID(db.Reader(), &sporkRootBlockID)
	if err != nil {
		return nil, irrecoverable.NewExceptionf("could not get spork root block ID: %w", err)
	}

	sporkRootBlock, err := blocks.ByID(sporkRootBlockID)
	if err != nil {
		return nil, irrecoverable.NewExceptionf("could not retrieve spork root block: %w", err)
	}

	return sporkRootBlock, nil
}
