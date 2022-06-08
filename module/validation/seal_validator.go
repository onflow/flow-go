package validation

import (
	"fmt"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/fork"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// DefaultRequiredApprovalsForSealValidation is the default number of approvals that should be
// present and valid for each chunk. Setting this to 0 will disable counting of chunk approvals
// this can be used temporarily to ease the migration to new chunk based sealing.
// TODO:
//   * This value is for the happy path (requires just one approval per chunk).
//   * Full protocol should be +2/3 of all currently authorized verifiers.
const DefaultRequiredApprovalsForSealValidation = 0

// sealValidator holds all needed context for checking seal
// validity against current protocol state.
type sealValidator struct {
	state                                protocol.State
	assigner                             module.ChunkAssigner
	signatureHasher                      hash.Hasher
	seals                                storage.Seals
	headers                              storage.Headers
	index                                storage.Index
	results                              storage.ExecutionResults
	requiredApprovalsForSealConstruction uint // number of required approvals per chunk to construct a seal
	requiredApprovalsForSealVerification uint // number of required approvals per chunk for a seal to be valid
	metrics                              module.ConsensusMetrics
}

func NewSealValidator(
	state protocol.State,
	headers storage.Headers,
	index storage.Index,
	results storage.ExecutionResults,
	seals storage.Seals,
	assigner module.ChunkAssigner,
	requiredApprovalsForSealConstruction uint,
	requiredApprovalsForSealVerification uint,
	metrics module.ConsensusMetrics,
) (*sealValidator, error) {
	if requiredApprovalsForSealConstruction < requiredApprovalsForSealVerification {
		return nil, fmt.Errorf("required number of approvals for seal construction (%d) cannot be smaller than for seal verification (%d)",
			requiredApprovalsForSealConstruction, requiredApprovalsForSealVerification)
	}

	return &sealValidator{
		state:                                state,
		assigner:                             assigner,
		signatureHasher:                      crypto.NewBLSKMAC(encoding.ResultApprovalTag),
		headers:                              headers,
		results:                              results,
		seals:                                seals,
		index:                                index,
		requiredApprovalsForSealConstruction: requiredApprovalsForSealConstruction,
		requiredApprovalsForSealVerification: requiredApprovalsForSealVerification,
		metrics:                              metrics,
	}, nil
}

func (s *sealValidator) verifySealSignature(aggregatedSignatures *flow.AggregatedSignature,
	chunk *flow.Chunk, executionResultID flow.Identifier) error {
	// TODO: replace implementation once proper aggregation is used for Verifiers' attestation signatures.

	atst := flow.Attestation{
		BlockID:           chunk.BlockID,
		ExecutionResultID: executionResultID,
		ChunkIndex:        chunk.Index,
	}
	atstID := atst.ID()

	for i, signature := range aggregatedSignatures.VerifierSignatures {
		signerId := aggregatedSignatures.SignerIDs[i]

		nodeIdentity, err := identityForNode(s.state, chunk.BlockID, signerId)
		if err != nil {
			return err
		}

		valid, err := nodeIdentity.StakingPubKey.Verify(signature, atstID[:], s.signatureHasher)
		if err != nil {
			return fmt.Errorf("failed to verify signature: %w", err)
		}

		if !valid {
			return engine.NewInvalidInputErrorf("Invalid signature for (%x)", nodeIdentity.NodeID)
		}
	}

	return nil
}

// Validate checks the compliance of the payload seals and returns the last
// valid seal on the fork up to and including `candidate`. To be valid, we
// require that seals
// 1) form a valid chain on top of the last seal as of the parent of `candidate` and
// 2) correspond to blocks and execution results incorporated on the current fork.
// 3) has valid signatures for all of its chunks.
//
// Note that we don't explicitly check that sealed results satisfy the sub-graph
// check. Nevertheless, correctness in this regard is guaranteed because:
//  * We only allow seals that correspond to ExecutionReceipts that were
//    incorporated in this fork.
//  * We only include ExecutionReceipts whose results pass the sub-graph check
//    (as part of ReceiptValidator).
// => Therefore, only seals whose results pass the sub-graph check will be
//    allowed.
func (s *sealValidator) Validate(candidate *flow.Block) (*flow.Seal, error) {
	header := candidate.Header
	payload := candidate.Payload
	parentID := header.ParentID

	// Get the latest seal in the fork that ends with the candidate's parent.
	// The protocol state saves this information for each block that has been
	// successfully added to the chain tree (even when the added block does not
	// itself contain a seal). Per prerequisite of this method, the candidate block's parent must
	// be part of the main chain (without any missing ancestors). For every block B that is
	// attached to the main chain, we store the latest seal in the fork that ends with B.
	// Therefore, _not_ finding the latest sealed block of the parent constitutes
	// a fatal internal error.
	lastSealUpToParent, err := s.seals.ByBlockID(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve parent seal (%x): %w", parentID, err)
	}

	// if there is no seal in the block payload, use the last sealed block of
	// the parent block as the last sealed block of the given block.
	if len(payload.Seals) == 0 {
		return lastSealUpToParent, nil
	}

	// map each seal to the block it is sealing for easy lookup; we will need to
	// successfully connect _all_ of these seals to the last sealed block for
	// the payload to be valid
	byBlock := make(map[flow.Identifier]*flow.Seal)
	for _, seal := range payload.Seals {
		byBlock[seal.BlockID] = seal
	}
	if len(payload.Seals) != len(byBlock) {
		return nil, engine.NewInvalidInputError("multiple seals for the same block")
	}

	// incorporatedResults collects execution results that are incorporated in unsealed
	// blocks; CAUTION: some of these incorporated results might already be sealed.
	incorporatedResults := make(map[flow.Identifier]*flow.IncorporatedResult)

	// IDs of unsealed blocks on the fork
	var unsealedBlockIDs []flow.Identifier

	// Traverse fork starting from the lowest unsealed block (included) up to the parent block (included).
	// For each visited block collect: IncorporatedResults and block ID
	forkCollector := func(header *flow.Header) error {
		blockID := header.ID()
		if blockID == parentID {
			// Important protocol edge case: There must be at least one block in between the block incorporating
			// a result and the block sealing the result. This is because we need the Source of Randomness for
			// the block that _incorporates_ the result, to compute the verifier assignment. Therefore, we require
			// that the block _incorporating_ the result has at least one child in the fork, _before_ we include
			// the seal. Thereby, we guarantee that a verifier assignment can be computed without needing
			// information from the block that we are just constructing. Hence, we don't allow results to be
			// sealed that were incorporated in the immediate parent which is being extended.
			return nil
		}

		// keep track of blocks on the fork
		unsealedBlockIDs = append(unsealedBlockIDs, blockID)

		// Collect incorporated results
		payloadIndex, err := s.index.ByBlockID(blockID)
		if err != nil {
			return fmt.Errorf("could not get block payload %x: %w", blockID, err)
		}
		for _, resultID := range payloadIndex.ResultIDs {
			result, err := s.results.ByID(resultID)
			if err != nil {
				return fmt.Errorf("internal error fetching result %v incorporated in block %v: %w", resultID, blockID, err)
			}
			incorporatedResults[resultID] = flow.NewIncorporatedResult(blockID, result)
		}
		return nil
	}
	err = fork.TraverseForward(s.headers, parentID, forkCollector, fork.ExcludingBlock(lastSealUpToParent.BlockID))
	if err != nil {
		return nil, fmt.Errorf("internal error collecting incorporated results from unsealed fork: %w", err)
	}

	// We do _not_ add the results from the candidate block's own payload to incorporatedResults.
	// That's because a result requires to be added to a bock first in order to determine
	// its chunk assignment for verification. Therefore a seal can only be added in the
	// next block or after. In other words, a receipt and its seal can't be the same block.

	// Iterate through the unsealed blocks, starting at the one with lowest
	// height and try to create a chain of valid seals.
	latestSeal := lastSealUpToParent
	for _, blockID := range unsealedBlockIDs {
		// if there are no more seals left, we can exit earlier
		if len(byBlock) == 0 {
			return latestSeal, nil
		}

		// the chain of seals should not skip blocks
		seal, found := byBlock[blockID]
		if !found {
			return nil, engine.NewInvalidInputErrorf("chain of seals broken (missing seal for block %x)", blockID)
		}
		delete(byBlock, blockID)

		// the sealed result must be previously incorporated in the fork:
		incorporatedResult, ok := incorporatedResults[seal.ResultID]
		if !ok {
			return nil, engine.NewInvalidInputErrorf("seal %x does not correspond to a result on this fork", seal.ID())
		}

		// check the integrity of the seal (by itself)
		err := s.validateSeal(seal, incorporatedResult)
		if err != nil {
			if !engine.IsInvalidInputError(err) {
				return nil, fmt.Errorf("unexpected internal error while validating seal %x for result %x for block %x: %w",
					seal.ID(), seal.ResultID, seal.BlockID, err)
			}
			return nil, fmt.Errorf("invalid seal %x for result %x for block %x: %w", seal.ID(), seal.ResultID, seal.BlockID, err)
		}

		// check that the sealed execution results form a chain
		if incorporatedResult.Result.PreviousResultID != latestSeal.ResultID {
			return nil, engine.NewInvalidInputErrorf("sealed execution results for block %x does not connect to previously sealed result", blockID)
		}

		latestSeal = seal
	}

	// it is illegal to include more seals than there are unsealed blocks in the fork
	if len(byBlock) > 0 {
		return nil, engine.NewInvalidInputErrorf("more seals then unsealed blocks in fork (left: %d)", len(byBlock))
	}

	return latestSeal, nil
}

// validateSeal performs integrity checks of single seal. To be valid, we
// require that seal:
// 1) Contains correct number of approval signatures, one aggregated sig for each chunk.
// 2) Every aggregated signature contains valid signer ids. module.ChunkAssigner is used to perform this check.
// 3) Every aggregated signature contains valid signatures.
// Returns:
// * nil - in case of success
// * engine.InvalidInputError - in case of malformed seal
// * exception - in case of unexpected error
func (s *sealValidator) validateSeal(seal *flow.Seal, incorporatedResult *flow.IncorporatedResult) error {
	executionResult := incorporatedResult.Result

	// check that each chunk has an AggregatedSignature
	if len(seal.AggregatedApprovalSigs) != executionResult.Chunks.Len() {
		return engine.NewInvalidInputErrorf("mismatching signatures, expected: %d, got: %d",
			executionResult.Chunks.Len(),
			len(seal.AggregatedApprovalSigs))
	}

	assignments, err := s.assigner.Assign(executionResult, incorporatedResult.IncorporatedBlockID)
	if err != nil {
		return fmt.Errorf("failed to retrieve verifier assignment for result %x incorporated in block %x: %w",
			executionResult.ID(), incorporatedResult.IncorporatedBlockID, err)
	}

	// Check that each AggregatedSignature has enough valid signatures from
	// verifiers that were assigned to the corresponding chunk.
	executionResultID := executionResult.ID()
	emergencySealed := false
	for _, chunk := range executionResult.Chunks {
		chunkSigs := &seal.AggregatedApprovalSigs[chunk.Index]

		// for each approving Verification Node (SignerID), we expect exactly one signature
		numberApprovers := chunkSigs.CardinalitySignerSet()
		if len(chunkSigs.SignerIDs) != numberApprovers {
			return engine.NewInvalidInputErrorf("chunk %d contains repeated approvals from the same verifier", chunk.Index)
		}
		if len(chunkSigs.VerifierSignatures) != numberApprovers {
			return engine.NewInvalidInputErrorf("expecting signatures from %d approvers but got %d", numberApprovers, len(chunkSigs.VerifierSignatures))
		}

		// the chunk must have been approved by at least the minimally
		// required number of Verification Nodes
		if uint(numberApprovers) < s.requiredApprovalsForSealConstruction {
			if uint(numberApprovers) >= s.requiredApprovalsForSealVerification {
				// Emergency sealing is a _temporary_ fallback to reduce the probability of
				// sealing halts due to bugs in the verification nodes, where they don't
				// approve a chunk even though they should (false-negative).
				// TODO: remove this fallback for BFT
				emergencySealed = true
			} else {
				return engine.NewInvalidInputErrorf("chunk %d has %d approvals but require at least %d",
					chunk.Index, numberApprovers, s.requiredApprovalsForSealVerification)
			}
		}

		// only Verification Nodes that were assigned to the chunk are allowed to approve it
		for _, signerId := range chunkSigs.SignerIDs {
			if !assignments.HasVerifier(chunk, signerId) {
				return engine.NewInvalidInputErrorf("invalid signer id at chunk: %d", chunk.Index)
			}
		}

		// Verification Nodes' approval signatures must be valid
		err := s.verifySealSignature(chunkSigs, chunk, executionResultID)
		if err != nil {
			return fmt.Errorf("invalid seal signature: %w", err)
		}
	}

	// TODO: remove this metric after emergency-sealing development
	if emergencySealed {
		s.metrics.EmergencySeal()
	}

	return nil
}
