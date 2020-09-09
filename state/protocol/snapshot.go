// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package protocol

import (
	"encoding/binary"
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/signature"
)

// Snapshot represents an immutable snapshot at a specific point of the
// protocol state history. It allows us to read the parameters at the selected
// point in a deterministic manner.
type Snapshot interface {

	// Identities returns a list of identities at the selected point of
	// the protocol state history. It allows us to provide optional upfront
	// filters which can be used by the implementation to speed up database
	// lookups.
	Identities(selector flow.IdentityFilter) (flow.IdentityList, error)

	// Identity attempts to retrieve the node with the given identifier at the
	// selected point of the protocol state history. It will error if it doesn't exist.
	Identity(nodeID flow.Identifier) (*flow.Identity, error)

	// Commit return the sealed execution state commitment at this block.
	Commit() (flow.StateCommitment, error)

	// Head returns the latest block at the selected point of the protocol state
	// history. It can represent either a finalized or ambiguous block,
	// depending on our selection criteria. Either way, it's the block on which
	// we should build the next block in the context of the selected state.
	Head() (*flow.Header, error)

	// Pending returns all children IDs for the snapshot head, which thus were
	// potential extensions of the protocol state at this snapshot. The result
	// is ordered such that parents are included before their children. These
	// are NOT guaranteed to have been validated by HotStuff.
	Pending() ([]flow.Identifier, error)

	// Seed returns the random seed for a certain snapshot.
	// In order to deterministically derive task specific seeds, indices must
	// be specified.
	// Refer to module/indices/rand.go for different indices.
	// NOTE: not to be confused with the epoch seed.
	Seed(indices ...uint32) ([]byte, error)

	// Epoch will return the counter of the epoch for the given snapshot.
	Epoch() (uint64, error)
}

// SeedFromParentSignature reads the raw random seed from a combined signature.
// the combinedSig must be from a QuorumCertificate. The indices can be used to
// generate task-specific seeds from the same signature.
func SeedFromParentSignature(indices []uint32, combinedSig crypto.Signature) ([]byte, error) {
	if len(indices)*4 > hash.KmacMaxParamsLen {
		return nil, fmt.Errorf("unsupported number of indices")
	}

	// create the key used for the KMAC by concatenating all indices
	key := make([]byte, 4*len(indices))
	for i, index := range indices {
		binary.LittleEndian.PutUint32(key[4*i:4*i+4], index)
	}

	// create a KMAC instance with our key and 32 bytes output size
	kmac, err := hash.NewKMAC_128(key, nil, 32)
	if err != nil {
		return nil, fmt.Errorf("could not create kmac: %w", err)
	}

	// split the parent voter sig into staking & beacon parts
	combiner := signature.NewCombiner()
	sigs, err := combiner.Split(combinedSig)
	if err != nil {
		return nil, fmt.Errorf("could not split block signature: %w", err)
	}
	if len(sigs) != 2 {
		return nil, fmt.Errorf("invalid block signature split")
	}

	// generate the seed by hashing the random beacon threshold signature
	beaconSig := sigs[1]
	seed := kmac.ComputeHash(beaconSig)

	return seed, nil
}
