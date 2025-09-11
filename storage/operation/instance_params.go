package operation

import (
	"fmt"

	"github.com/jordanschalm/lockctx"
	"github.com/vmihailenco/msgpack/v4"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

const DefaultInstanceParamsVersion = 0

// VersionedInstanceParams wraps instance params with a version tag.
type VersionedInstanceParams struct {
	Version        uint64
	InstanceParams interface{}
}

// NewVersionedInstanceParams constructs an instance params for a particular version for bootstrapping.
//
// No errors are expected during normal operation.
func NewVersionedInstanceParams(
	version uint64,
	finalizedRootID flow.Identifier,
	sealedRootID flow.Identifier,
	sporkRootBlockID flow.Identifier,
) (*VersionedInstanceParams, error) {
	versionedInstanceParams := &VersionedInstanceParams{
		Version: version,
	}
	switch version {
	case 0:
		versionedInstanceParams.InstanceParams = InstanceParamsV0{
			FinalizedRootID:  finalizedRootID,
			SealedRootID:     sealedRootID,
			SporkRootBlockID: sporkRootBlockID,
		}
	default:
		return nil, fmt.Errorf("unsupported instance params version: %d", version)
	}

	return versionedInstanceParams, nil
}

func (v *VersionedInstanceParams) UnmarshalMsgpack(b []byte) error {
	// alias type to prevent recursion
	type decodable VersionedInstanceParams
	var d decodable
	if err := msgpack.Unmarshal(b, &d); err != nil {
		return fmt.Errorf("could not decode VersionedInstanceParams: %w", err)
	}

	v.Version = d.Version
	instanceBytes, err := msgpack.Marshal(d.InstanceParams)
	if err != nil {
		return fmt.Errorf("could not re-marshal InstanceParams: %w", err)
	}

	// decode InstanceParams based on version
	switch d.Version {
	case 0:
		var p InstanceParamsV0
		if err := msgpack.Unmarshal(instanceBytes, &p); err != nil {
			return fmt.Errorf("could not decode to InstanceParamsV0: %w", err)
		}
		v.InstanceParams = p
	default:
		return fmt.Errorf("unsupported instance params version: %d", d.Version)
	}

	return nil
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

// InsertInstanceParams stores the consolidated instance params under a single key.
//
// CAUTION:
//   - This function is intended to be called exactly once during bootstrapping.
//   - Overwrites are prevented by an explicit existence check; if data is already present, error is returned.
//
// Expected errors during normal operations:
//   - [storage.ErrAlreadyExists] if instance params have already been stored.
//   - Generic error for unexpected database or encoding failures.
func InsertInstanceParams(lctx lockctx.Proof, rw storage.ReaderBatchWriter, params VersionedInstanceParams) error {
	if !lctx.HoldsLock(storage.LockBootstrapping) {
		return fmt.Errorf("missing required lock: %s", storage.LockBootstrapping)
	}
	key := MakePrefix(codeInstanceParams)
	exist, err := KeyExists(rw.GlobalReader(), key)
	if err != nil {
		return err
	}
	if exist {
		return fmt.Errorf("instance params is already stored: %w", storage.ErrAlreadyExists)
	}
	return UpsertByKey(rw.Writer(), key, params)
}

// RetrieveInstanceParams retrieves the consolidated instance params from storage.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if the key does not exist (not bootstrapped).
//   - Generic error for unexpected database or decoding failures.
func RetrieveInstanceParams(r storage.Reader, params *VersionedInstanceParams) error {
	return RetrieveByKey(r, MakePrefix(codeInstanceParams), params)
}
