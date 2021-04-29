package validation

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/engine"
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
//   * Full protocol should be +2/3 of all currently staked verifiers.
const DefaultRequiredApprovalsForSealValidation = 0

// sealValidator holds all needed context for checking seal
// validity against current protocol state.
type sealValidator struct {
	state                                protocol.State
	assigner                             module.ChunkAssigner
	verifier                             module.Verifier
	seals                                storage.Seals
	headers                              storage.Headers
	index                                storage.Index
	results                              storage.ExecutionResults
	requiredApprovalsForSealVerification uint
	metrics                              module.ConsensusMetrics
}

func NewSealValidator(state protocol.State, headers storage.Headers, index storage.Index, results storage.ExecutionResults, seals storage.Seals,
	assigner module.ChunkAssigner, verifier module.Verifier, requiredApprovalsForSealVerification uint, metrics module.ConsensusMetrics) *sealValidator {

	rv := &sealValidator{
		state:                                state,
		assigner:                             assigner,
		verifier:                             verifier,
		headers:                              headers,
		results:                              results,
		seals:                                seals,
		index:                                index,
		requiredApprovalsForSealVerification: requiredApprovalsForSealVerification,
		metrics:                              metrics,
	}

	return rv
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

		valid, err := s.verifier.Verify(atstID[:], signature, nodeIdentity.StakingPubKey)
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

	// Get the latest seal in the fork that ends with the candidate's parent.
	// The protocol state saves this information for each block that has been
	// successfully added to the chain tree (even when the added block does not
	// itself contain a seal). Per prerequisite of this method, the candidate block's parent must
	// be part of the main chain (without any missing ancestors). For every block B that is
	// attached to the main chain, we store the latest seal in the fork that ends with B.
	// Therefore, _not_ finding the latest sealed block of the parent constitutes
	// a fatal internal error.
	lastSealUpToParent, err := s.seals.ByBlockID(header.ParentID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve parent seal (%x): %w", candidate.Header.ParentID, err)
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
				return fmt.Errorf("internal error fetching result %v incorporated in stored block %v: %w", resultID, blockID, err)
			}
			// ATTENTION:
			// Here, IncorporatedBlockID (the first argument) should be set
			// to ancestorID, because that is the block that contains the
			// ExecutionResult. However, in phase 2 of the sealing roadmap,
			// we are still using a temporary sealing logic where the
			// IncorporatedBlockID is expected to be the result's block ID.
			incorporatedResults[resultID] = flow.NewIncorporatedResult(result.BlockID, result)
		}
		return nil
	}
	err = fork.TraverseForward(s.headers, header.ParentID, forkCollector, fork.ExcludingBlock(lastSealUpToParent.BlockID))
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
			if engine.IsInvalidInputError(err) {
				// Skip fail on an invalid seal. We don't put this earlier in the function
				// because we still want to test that the above code doesn't panic.
				// TODO: this is only here temporarily to ease the migration to chunk-based sealing.
				if s.requiredApprovalsForSealVerification == 0 {
					log.Warn().Msgf("payload includes invalid seal, continuing validation (%x): %s", seal.ID(), err.Error())
				} else {
					return nil, fmt.Errorf("payload includes invalid seal (%x): %w", seal.ID(), err)
				}
			} else {
				return nil, fmt.Errorf("unexpected seal validation error: %w", err)
			}
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
		// this is not an error if we don't require any approvals
		if s.requiredApprovalsForSealVerification == 0 {
			// TODO: remove this metric after emergency-sealing development
			// phase. Here we assume that the seal was created in emergency-mode
			// (because the flag required-contruction-seal-approvals is > 0),
			// so we increment the related metric and accept the seal.
			s.metrics.EmergencySeal()
			return nil
		}

		return engine.NewInvalidInputErrorf("mismatching signatures, expected: %d, got: %d",
			executionResult.Chunks.Len(),
			len(seal.AggregatedApprovalSigs))
	}

	assignments, err := s.assigner.Assign(executionResult, incorporatedResult.IncorporatedBlockID)
	if err != nil {
		return fmt.Errorf("could not retreive assignments for block: %v, %w", seal.BlockID, err)
	}

	// Check that each AggregatedSignature has enough valid signatures from
	// verifiers that were assigned to the corresponding chunk.
	executionResultID := executionResult.ID()
	for _, chunk := range executionResult.Chunks {
		chunkSigs := &seal.AggregatedApprovalSigs[chunk.Index]
		numberApprovals := len(chunkSigs.SignerIDs)
		if uint(numberApprovals) < s.requiredApprovalsForSealVerification {
			return engine.NewInvalidInputErrorf("not enough chunk approvals %d vs %d",
				numberApprovals, s.requiredApprovalsForSealVerification)
		}

		lenVerifierSigs := len(chunkSigs.VerifierSignatures)
		if lenVerifierSigs != numberApprovals {
			return engine.NewInvalidInputErrorf("mismatched signatures length %d vs %d",
				lenVerifierSigs, numberApprovals)
		}

		for _, signerId := range chunkSigs.SignerIDs {
			if !assignments.HasVerifier(chunk, signerId) {
				return engine.NewInvalidInputErrorf("invalid signer id at chunk: %d", chunk.Index)
			}
		}

		err := s.verifySealSignature(chunkSigs, chunk, executionResultID)
		if err != nil {
			return fmt.Errorf("invalid seal signature: %w", err)
		}
	}

	return nil
}
