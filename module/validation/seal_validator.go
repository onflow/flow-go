package validation

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
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
	payloads                             storage.Payloads
	requiredApprovalsForSealVerification uint
	metrics                              module.ConsensusMetrics
}

func NewSealValidator(state protocol.State, headers storage.Headers, payloads storage.Payloads, seals storage.Seals,
	assigner module.ChunkAssigner, verifier module.Verifier, requiredApprovalsForSealVerification uint, metrics module.ConsensusMetrics) *sealValidator {

	rv := &sealValidator{
		state:                                state,
		assigner:                             assigner,
		verifier:                             verifier,
		headers:                              headers,
		seals:                                seals,
		payloads:                             payloads,
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
	// Get the latest seal in the fork that ends with the candidate's parent.
	// The protocol state saves this information for each block that has been
	// successfully added to the chain tree (even when the added block does not
	// itself contain a seal). Per prerequisite of this method, the candidate block's parent must
	// be part of the main chain (without any missing ancestors). For every block B that is
	// attached to the main chain, we store the latest seal in the fork that ends with B.
	// Therefore, _not_ finding the latest sealed block of the parent constitutes
	// a fatal internal error.
	lastSealUpToParent, err := s.seals.ByBlockID(candidate.Header.ParentID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve parent seal (%x): %w", candidate.Header.ParentID, err)
	}

	header := candidate.Header
	payload := candidate.Payload

	last := lastSealUpToParent

	// if there is no seal in the block payload, use the last sealed block of
	// the parent block as the last sealed block of the given block.
	if len(payload.Seals) == 0 {
		return last, nil
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

	// unsealedResults collects the _first_ appearance of unsealed execution
	// results on the fork, along with the ID of the block in which they are
	// incorporated.
	unsealedResults := make(map[flow.Identifier]*flow.IncorporatedResult)

	// collect IDs of blocks on the fork (from parent to last sealed)
	var blockIDs []flow.Identifier

	// loop through the fork backwards from the parent block
	// up to last sealed block and collect
	// IncorporatedResults as well as the IDs of blocks visited
	sealedID := last.BlockID
	ancestorID := header.ParentID
	for ancestorID != sealedID {

		ancestor, err := s.headers.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve ancestor header (%x): %w", ancestorID, err)
		}

		// keep track of blocks on the fork
		blockIDs = append(blockIDs, ancestorID)

		payload, err := s.payloads.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not get block payload %x: %w", ancestorID, err)
		}

		// Collect execution results from receipts.
		for _, receipt := range payload.Receipts {
			resultID := receipt.ExecutionResult.ID()
			// unsealedResults should contain the _first_ appearance of the
			// ExecutionResult. We are traversing the fork backwards, so we can
			// overwrite any previously recorded result.

			// ATTENTION:
			// Here, IncorporatedBlockID (the first argument) should be set
			// to ancestorID, because that is the block that contains the
			// ExecutionResult. However, in phase 2 of the sealing roadmap,
			// we are still using a temporary sealing logic where the
			// IncorporatedBlockID is expected to be the result's block ID.
			unsealedResults[resultID] = flow.NewIncorporatedResult(
				receipt.ExecutionResult.BlockID,
				&receipt.ExecutionResult,
			)

		}

		ancestorID = ancestor.ParentID
	}

	// we do not include the receipts in the same payload to the unsealedResults.
	// that's because a result requires to be added to a bock first in order to determine
	// its chunk assignment for verification, therefore a seal can only be added in
	// the next block or after. In other words, a receipt and its seal can't be
	// added in the same block.

	// Loop forward across the fork and try to create a valid chain of seals.
	// blockIDs, as populated by the previous loop, is in reverse order.
	for i := len(blockIDs) - 1; i >= 0; i-- {
		// if there are no more seals left, we can exit earlier
		if len(byBlock) == 0 {
			return last, nil
		}

		// return an error if there are still seals to consider, but they break
		// the chain
		blockID := blockIDs[i]
		seal, found := byBlock[blockID]
		if !found {
			return nil, engine.NewInvalidInputErrorf("chain of seals broken (missing: %x)", blockID)
		}

		delete(byBlock, blockID)

		// check if we have an incorporatedResult for this seal
		incorporatedResult, ok := unsealedResults[seal.ResultID]
		if !ok {
			return nil, engine.NewInvalidInputErrorf("seal %x does not correspond to a result on this fork", seal.ID())
		}

		// check the integrity of the seal
		err := s.validateSeal(seal, incorporatedResult)
		if err != nil {
			if engine.IsInvalidInputError(err) {
				// Skip fail on an invalid seal. We don't put this earlier in the function
				// because we still want to test that the above code doesn't panic.
				// TODO: this is only here temporarily to ease the migration to new chunk
				// based sealing.
				if s.requiredApprovalsForSealVerification == 0 {
					log.Warn().Msgf("payload includes invalid seal, continuing validation (%x): %s", seal.ID(), err.Error())
				} else {
					return nil, fmt.Errorf("payload includes invalid seal (%x): %w", seal.ID(), err)
				}
			} else {
				return nil, fmt.Errorf("unexpected seal validation error: %w", err)
			}
		}

		last = seal
	}

	// at this point no seals should be left
	if len(byBlock) > 0 {
		return nil, engine.NewInvalidInputErrorf("not all seals connected to state (left: %d)", len(byBlock))
	}

	return last, nil
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
