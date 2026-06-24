package datastore

import (
	"fmt"

	"github.com/vmihailenco/msgpack/v4"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

const DefaultInstanceParamsVersion = 0

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
	// sporkRoot is the root block for the present spork.
	sporkRootBlock *flow.Block
}

var _ protocol.InstanceParams = (*InstanceParams)(nil)

// ReadInstanceParams reads the instance parameters from the database and returns them as in-memory representation.
// It serves as a constructor for InstanceParams and only requires read-only access to the database (we never write).
// This information is immutable for the lifetime of a node and may be cached.
// No errors are expected during normal operation.
func ReadInstanceParams(
	r storage.Reader,
	headers storage.Headers,
	seals storage.Seals,
	blocks storage.Blocks,
) (*InstanceParams, error) {
	params := &InstanceParams{}

	// The values below are written during bootstrapping and immutable for the lifetime of the node. All
	// following parameters are uniquely defined by the values initially read. No atomicity is required.
	var versioned flow.VersionedInstanceParams
	err := operation.RetrieveInstanceParams(r, &versioned)
	if err != nil {
		return nil, fmt.Errorf("could not read instance params to populate cache: %w", err)
	}

	switch versioned.Version {
	case 0:
		var v0 InstanceParamsV0
		if err := msgpack.Unmarshal(versioned.Data, &v0); err != nil {
			return nil, fmt.Errorf("could not decode to InstanceParamsV0: %w", err)
		}
		params.finalizedRoot, err = headers.ByBlockID(v0.FinalizedRootID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve finalized root header: %w", err)
		}

		params.sealedRoot, err = headers.ByBlockID(v0.SealedRootID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve sealed root header: %w", err)
		}

		// retrieve the root seal
		params.rootSeal, err = seals.HighestInFork(v0.FinalizedRootID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve root seal: %w", err)
		}

		params.sporkRootBlock, err = blocks.ByID(v0.SporkRootBlockID)
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

// SporkRootBlock returns the root block for the present spork.
func (p *InstanceParams) SporkRootBlock() *flow.Block {
	return p.sporkRootBlock
}

// InstanceParamsV0 is the consolidated, serializable form of protocol instance
// parameters that are constant throughout the lifetime of a node.
type InstanceParamsV0 struct {
	// FinalizedRootID is the ID of the finalized root block.
	FinalizedRootID flow.Identifier
	// SealedRootID is the ID of the sealed root block.
	SealedRootID flow.Identifier
	// SporkRootBlockID is the root block's ID for the present spork this node participates in.
	SporkRootBlockID flow.Identifier
}

// NewVersionedInstanceParams constructs a versioned binary blob representing the `InstanceParams`.
// Conceptually, the values in the `InstanceParams` are immutable during the lifetime of a node.
// However, versioning allows extending `InstanceParams` with new fields in the future.
//
// No errors are expected during normal operation.
func NewVersionedInstanceParams(
	version uint64,
	finalizedRootID flow.Identifier,
	sealedRootID flow.Identifier,
	sporkRootBlockID flow.Identifier,
) (*flow.VersionedInstanceParams, error) {
	versionedInstanceParams := &flow.VersionedInstanceParams{
		Version: version,
	}
	var data interface{}
	switch version {
	case 0:
		data = InstanceParamsV0{
			FinalizedRootID:  finalizedRootID,
			SealedRootID:     sealedRootID,
			SporkRootBlockID: sporkRootBlockID,
		}
	default:
		return nil, fmt.Errorf("unsupported instance params version: %d", version)
	}

	encodedData, err := msgpack.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("could not encode InstanceParams: %w", err)
	}
	versionedInstanceParams.Data = encodedData

	return versionedInstanceParams, nil
}
