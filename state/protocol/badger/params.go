package badger

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
	// sporkRoot is the root block for the present spork.
	sporkRootBlock *flow.Block
}

var _ protocol.InstanceParams = (*InstanceParams)(nil)

// ReadInstanceParams reads the instance parameters from the database and returns them as in-memory representation.
// It serves as a constructor for InstanceParams and only requires a read-only database handle,
// emphasizing that it only reads and never writes.
// This information is immutable and may be cached.
// No errors are expected during normal operation.
func ReadInstanceParams(
	r storage.Reader,
	headers storage.Headers,
	seals storage.Seals,
	blocks storage.Blocks,
) (*InstanceParams, error) {
	params := &InstanceParams{}
	var versioned operation.VersionedInstanceParams
	err := operation.RetrieveInstanceParams(r, &versioned)
	if err != nil {
		return nil, fmt.Errorf("could not read instance params to populate cache: %w", err)
	}

	switch versioned.Version {
	case 0:
		enc, ok := versioned.InstanceParams.(operation.InstanceParamsV0)
		if !ok {
			return nil, fmt.Errorf("invalid instance params %T for version %d", versioned.InstanceParams, versioned.Version)
		}
		params.finalizedRoot, err = headers.ByBlockID(enc.FinalizedRootID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve finalized root header: %w", err)
		}

		params.sealedRoot, err = headers.ByBlockID(enc.SealedRootID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve sealed root header: %w", err)
		}

		// retrieve the root seal
		params.rootSeal, err = seals.HighestInFork(enc.FinalizedRootID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve root seal: %w", err)
		}

		params.sporkRootBlock, err = blocks.ByID(enc.SporkRootBlockID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve spork root block: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported instance params version: %d", versioned.Version)
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
