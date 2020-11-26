package badger

import (
	"errors"
	"fmt"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"

	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

func validSetup(setup *flow.EpochSetup) error {
	// STEP 1: general sanity checks
	// the seed needs to be at least minimum length
	if len(setup.RandomSource) < crypto.MinSeedLength {
		return fmt.Errorf("seed has insufficient length (%d < %d)", len(setup.RandomSource), crypto.MinSeedLength)
	}

	// STEP 2: sanity checks of all nodes listed as participants
	// there should be no duplicate node IDs
	identLookup := make(map[flow.Identifier]struct{})
	for _, participant := range setup.Participants {
		_, ok := identLookup[participant.NodeID]
		if ok {
			return fmt.Errorf("duplicate node identifier (%x)", participant.NodeID)
		}
		identLookup[participant.NodeID] = struct{}{}
	}

	// there should be no duplicate node addresses
	addrLookup := make(map[string]struct{})
	for _, participant := range setup.Participants {
		_, ok := addrLookup[participant.Address]
		if ok {
			return fmt.Errorf("duplicate node address (%x)", participant.Address)
		}
		addrLookup[participant.Address] = struct{}{}
	}

	// there should be no nodes with zero stake
	// TODO: we might want to remove the following as we generally want to allow nodes with
	// zero weight in the protocol state.
	for _, participant := range setup.Participants {
		if participant.Stake == 0 {
			return fmt.Errorf("node with zero stake (%x)", participant.NodeID)
		}
	}

	// STEP 3: sanity checks for individual roles
	// IMPORTANT: here we remove all nodes with zero weight, as they are allowed to partake
	// in communication but not in respective node functions
	activeParticipants := setup.Participants.Filter(filter.HasStake(true))

	// we need at least one node of each role
	roles := make(map[flow.Role]uint)
	for _, participant := range activeParticipants {
		roles[participant.Role]++
	}
	if roles[flow.RoleConsensus] < 1 {
		return fmt.Errorf("need at least one consensus node")
	}
	if roles[flow.RoleCollection] < 1 {
		return fmt.Errorf("need at least one collection node")
	}
	if roles[flow.RoleExecution] < 1 {
		return fmt.Errorf("need at least one execution node")
	}
	if roles[flow.RoleVerification] < 1 {
		return fmt.Errorf("need at least one verification node")
	}

	// we need at least one collection cluster
	if len(setup.Assignments) == 0 {
		return fmt.Errorf("need at least one collection cluster")
	}

	// the collection cluster assignments need to be valid
	_, err := flow.NewClusterList(setup.Assignments, activeParticipants.Filter(filter.HasRole(flow.RoleCollection)))
	if err != nil {
		return fmt.Errorf("invalid cluster assignments: %w", err)
	}

	return nil
}

func validCommit(commit *flow.EpochCommit, setup *flow.EpochSetup) error {

	if len(setup.Assignments) != len(commit.ClusterQCs) {
		return fmt.Errorf("number of clusters (%d) does not number of QCs (%d)", len(setup.Assignments), len(commit.ClusterQCs))
	}

	// make sure we have a valid DKG public key
	if commit.DKGGroupKey == nil {
		return fmt.Errorf("missing DKG public group key")
	}

	participants := setup.Participants.Filter(filter.HasRole(flow.RoleConsensus))

	// make sure each participant of the epoch has a DKG entry
	for _, participant := range participants {
		_, exists := commit.DKGParticipants[participant.NodeID]
		if !exists {
			return fmt.Errorf("missing DKG participant data (%x)", participant.NodeID)
		}
	}

	// make sure that there is no extra data
	if len(participants) != len(commit.DKGParticipants) {
		return fmt.Errorf("DKG data contains extra entries")
	}

	return nil
}

type receiptValidator struct {
	state    protocol.ReadOnlyState
	index    storage.Index
	results  storage.ExecutionResults
	verifier module.Verifier
}

func NewReceiptValidator(state protocol.ReadOnlyState, index storage.Index, results storage.ExecutionResults) protocol.ReceiptValidator {
	rv := &receiptValidator{
		state:    state,
		index:    index,
		results:  results,
		verifier: signature.NewAggregationVerifier(encoding.ExecutionReceiptTag),
	}

	return rv
}

// checkIsStakedNodeWithRole checks whether, at the given block, `nodeID`
//   * is an authorized member of the network
//   * has _positive_ weight
//   * and has the expected role
// Returns the following errors:
//   * sentinel engine.InvalidInputError if any of the above-listed conditions are violated.
//   * generic error indicating a fatal internal bug
// Note: the method receives the block header as proof of its existence.
// Therefore, we consider the case where the respective block is unknown to the
// protocol state as a symptom of a fatal implementation bug.
func (v *receiptValidator) ensureStakedNodeWithRole(identity *flow.Identity, expectedRole flow.Role) error {

	// check that the origin is a verification node
	if identity.Role != expectedRole {
		return engine.NewInvalidInputErrorf("expected node %x to have identity %s but got %s", identity.NodeID, expectedRole, identity.Role)
	}

	// check if the identity has a stake
	if identity.Stake == 0 {
		return engine.NewInvalidInputErrorf("node has zero stake (%x)", identity.NodeID)
	}

	// TODO: check if node was ejected
	return nil
}

// identityForNode ensures that `nodeID` is an authorized member of the network
// at the given block and returns the corresponding node's full identity. 
// Error returns:
//   * sentinel engine.InvalidInputError is nodeID is NOT an authorized member of the network
//   * generic error indicating a fatal internal problem
func (v *receiptValidator) identityForNode(blockID flow.Identifier, nodeID flow.Identifier) (*flow.Identity, error) {
	// get the identity of the origin node
	identity, err := v.state.AtBlockID(blockID).Identity(nodeID)
	if err != nil {
		if protocol.IsIdentityNotFound(err) {
			return nil, engine.NewInvalidInputErrorf("unknown node identity: %w", err)
		}
		// unexpected exception
		return nil, fmt.Errorf("failed to retrieve node identity: %w", err)
	}

	return identity, nil
}

func (v *receiptValidator) verifySignature(receipt *flow.ExecutionReceipt, nodeIdentity *flow.Identity) error {
	id := receipt.ID()
	valid, err := v.verifier.Verify(id[:], receipt.ExecutorSignature, nodeIdentity.StakingPubKey)
	if err != nil {
		return fmt.Errorf("failed to verify signature: %w", err)
	}

	if !valid {
		return engine.NewInvalidInputErrorf("Invalid signature for (%x)", nodeIdentity.NodeID)
	}

	return nil
}

func (v *receiptValidator) verifyChunksFormat(result *flow.ExecutionResult) error {
	for index, chunk := range result.Chunks.Items() {
		if uint(index) != chunk.CollectionIndex {
			return fmt.Errorf("invalid CollectionIndex, expected %d got %d", index, chunk.CollectionIndex)
		}

		if chunk.BlockID != result.BlockID {
			return fmt.Errorf("invalid blockID, expected %s got %s", result.BlockID, chunk.BlockID)
		}
	}

	// we create one chunk per collection, plus the
	// system chunk. so we can check if the chunk number matches with the
	// number of guarantees plus one; this will ensure the execution receipt
	// cannot lie about having less chunks and having the remaining ones
	// approved
	requiredChunks := 1 // system chunk: must exist for block's ExecutionResult, even if block payload itself is empty

	index, err := v.index.ByBlockID(result.BlockID)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return err
		}
		// reaching this line means the block is empty, i.e. it has no payload => we expect only the system chunk
	} else {
		requiredChunks += len(index.CollectionIDs)
	}

	if result.Chunks.Len() != requiredChunks {
		return fmt.Errorf("invalid number of chunks, expected %d got %d", requiredChunks, result.Chunks.Len())
	}

	return nil
}

func (v *receiptValidator) verifyExecutionResult(result *flow.ExecutionResult) error {
	prevResult, err := v.results.ByID(result.PreviousResultID)
	if err != nil {
		return fmt.Errorf("no previous result ID")
	}

	block, err := v.state.AtBlockID(result.BlockID).Head()
	if err != nil {
		return fmt.Errorf("no block found %s %w", result.BlockID, err)
	}

	if prevResult.BlockID != block.ParentID {
		return fmt.Errorf("invalid block for previous result %s", prevResult.BlockID)
	}

	return nil
}

// Validate performs checks for ExecutionReceipt being valid or no.
// Checks performed:
// 	- can find stake and stake is positive
//	- signature is correct
//	- chunks are in correct format
// 	- execution result has a valid parent
// Returns nil if all checks passed successfully
func (v *receiptValidator) Validate(receipt *flow.ExecutionReceipt) error {
	identity, err := v.identityForNode(receipt.ExecutionResult.BlockID, receipt.ExecutorID)

	err = v.ensureStakedNodeWithRole(receipt.ExecutorID, identity, flow.RoleExecution)
	if err != nil {
		return err
	}

	err = v.verifySignature(receipt, identity)
	if err != nil {
		return err
	}

	err = v.verifyChunksFormat(&receipt.ExecutionResult)
	if err != nil {
		return err
	}

	err = v.verifyExecutionResult(&receipt.ExecutionResult)
	if err != nil {
		return err
	}

	return nil
}
