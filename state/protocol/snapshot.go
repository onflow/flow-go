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

// Snapshot represents an immutable snapshot of the protocol state
// at a specific block, denoted as the Head block.
// The Snapshot is fork-specific and only accounts for the information contained
// in blocks along this fork up to (including) Head.
// It allows us to read the parameters at the selected block in a deterministic manner.
type Snapshot interface {

	// Head returns the latest block at the selected point of the protocol state
	// history. It can represent either a finalized or ambiguous block,
	// depending on our selection criteria. Either way, it's the block on which
	// we should build the next block in the context of the selected state.
	Head() (*flow.Header, error)

	// Epoch returns the Epoch counter for the Head block.
	Epoch() (uint64, error)

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

	// Pending returns the IDs of all descendants of the Head block. The IDs
	// are ordered such that parents are included before their children. These
	// are NOT guaranteed to have been validated by HotStuff.
	Pending() ([]flow.Identifier, error)

	// RandomBeacon returns a deterministic seed for a pseudo random number generator.
	// The seed is derived from the Source of Randomness for the Head block.
	// In order to deterministically derive task specific seeds, indices must
	// be specified. Refer to module/indices/rand.go for different indices.
	// NOTE: not to be confused with the epoch seed.
	RandomBeacon(indices ...uint32) ([]byte, error)

	// EpochSnapshot returns an snapshot of all information for the specified Epoch,
	// which is available along the fork ending with the Head block.
	// CAUTION: at the moment, we only consider finalized information.
	// If the preparation for the specified epoch is still ongoing as of the Head block,
	// some of EpochSnapshot's methods will return errors.
	EpochSnapshot(counter uint64) EpochSnapshot
}

// EpochSnapshot contains the information specific to a certain Epoch (defined
// by the epoch Counter). Note that the Epoch preparation can differ along
// different forks. Therefore, an EpochSnapshot is tied to the Head of one
// specific fork and only accounts for the information contained in blocks
// along this fork up to (including) Head.
// An EpochSnapshot instance is constant and reports the identical information
// even if progress is made later and more information becomes available in
// subsequent blocks.
//
// Methods error if epoch preparation has not progressed far enough for
// this information to be determined by a finalized block.
// CAUTION: at the moment, we only consider finalized information.
type EpochSnapshot interface {

	// Counter returns the Epoch's counter.
	Counter() (uint64, error)

	// Head returns a block. An EpochSnapshot is tied to the Head (block)
	// of one specific fork and only accounts for the information contained
	// in blocks along this fork up to (including) Head.
	Head() (*flow.Header, error)

	// FinalView returns the largest view number which still belongs to this Epoch.
	FinalView() (uint64, error)

	// InitialIdentities returns the identities for this epoch as they were
	// specified in the EpochSetup Event.
	InitialIdentities(selector flow.IdentityFilter) (flow.IdentityList, error)

	Clustering() (flow.ClusterList, error)

	ClusterInformation(index uint32) (Cluster, error)

	DKG() (DKG, error)

	EpochSetupSeed(indices ...uint32) ([]byte, error)

	// Phase returns the epoch's preparation phase as of the Head block.
	// CAUTION: at the moment, we only consider finalized information.
	Phase() (EpochPhase, error)
}

// EpochPreparationState represents a state of the Epoch preparation protocol.
// An EpochPreparationState is always respective to a certain block, aka the
// Head block, and only accounts for information contained in blocks along
// this fork up to (including) Head.
// For example, we say that Epoch N is in state SetUp, iff:
//   * the EpochSetup service event is contained in Head or one of its ancestors
//   * the EpochCommitted event in _not_ in the fork up to (including) Head
//
// CAUTION: at the moment, we only consider finalized information.
type EpochPhase int

const (
	// Integer values: in our docs, we referred to the Staking Auction Phase as
	// 'Phase 0'. During Phase 0, no information about the upcoming Epoch is available
	// on the protocol level. Hence, we omit Phase 0 here.

	EpochSetupPhase     EpochPhase = 1 + iota // fork includes EpochSetup event (in seal) but _not_ EpochCommitted event
	EpochCommittedPhase                       // fork includes EpochSetup _and_ EpochCommitted events (in seals)
)

func (s EpochPhase) String() string {
	names := [...]string{
		"SetUp",
		"EpochCommittedPhase",
	}
	return names[s-1]
}

// SeedFromParentSignature reads the raw random seed from a combined signature.
// the combinedSig must be from a QuorumCertificate. The indices can be used to
// generate task-specific seeds from the same signature.
func SeedFromParentSignature(indices []uint32, combinedSig crypto.Signature) ([]byte, error) {
	// split the parent voter sig into staking & beacon parts
	combiner := signature.NewCombiner()
	sigs, err := combiner.Split(combinedSig)
	if err != nil {
		return nil, fmt.Errorf("could not split block signature: %w", err)
	}
	if len(sigs) != 2 {
		return nil, fmt.Errorf("invalid block signature split")
	}

	return SeedFromSourceOfRandomness(indices, sigs[1])
}

// SeedFromSourceOfRandomness generates a task-specific seed (task is determined by indices).
func SeedFromSourceOfRandomness(indices []uint32, sor []byte) ([]byte, error) {
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

	return kmac.ComputeHash(sor), nil
}
