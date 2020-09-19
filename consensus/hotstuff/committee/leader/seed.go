package leader

import (
	"bytes"
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state"
	"github.com/dapperlabs/flow-go/state/protocol"
	seed2 "github.com/dapperlabs/flow-go/state/protocol/seed"
)

// ReadSeed returns the random seed, used by the leader selection.
// There are a few cases when nodes startup:
// 1. Protocol state has no block after sporking
// 2. Protocol state has only the root block. The root block has no QC.
// 3. Protocol state has the root block and another valid block built on top of the root block.
// 4. Protocol state has crossed epoch, meaning the epoch block is not the root block stored on disk.
// For case 1-2, we will use read random seed from the rootQC, because protocol state
//     doesn't have a block that has the seed.
// For case 3-4, we will read the seed from both rootQC and protocol state, and do a
//     santity check to ensure they are identical.
func ReadSeed(indices []uint32, rootHeader *flow.Header, rootQC *flow.QuorumCertificate, st protocol.State) ([]byte, error) {
	seed, err := seed2.FromParentSignature(indices, rootQC.SigData)
	if err != nil {
		return nil, fmt.Errorf("could not read seed from root QC: %w", err)
	}

	snapshot := st.AtBlockID(rootHeader.ID())
	seedFromState, err := snapshot.Seed(indices...)

	// when the genesis has no child in protocol state yet, we can't get the seed.
	// In which case, we will use the seed from the root QC.
	if state.IsNoValidChildBlockError(err) {
		return seed, nil
	}

	if err != nil {
		return nil, fmt.Errorf("could not produce random seed: %w", err)
	}

	// if we have the seed from both root QC and protocol state, we will do
	// a sanity check here to make sure they are identical
	if !bytes.Equal(seed, seedFromState) {
		return nil, fmt.Errorf("sanity check that seed from root QC and state should be the same, but actually not. seed from root QC: %x, seed from state: %x",
			seed, seedFromState)
	}

	return seed, nil
}
