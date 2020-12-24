package validation

import (
	"fmt"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/storage"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
)

type sealValidator struct {
	state    protocol.State
	assigner module.ChunkAssigner
	verifier module.Verifier
	seals    storage.Seals
	headers  storage.Headers
	payloads storage.Payloads
}

func NewSealValidator(state protocol.State, headers storage.Headers, payloads storage.Payloads, seals storage.Seals, assigner module.ChunkAssigner, verifier module.Verifier) *sealValidator {
	rv := &sealValidator{
		state:    state,
		assigner: assigner,
		verifier: verifier,
		headers:  headers,
		seals:    seals,
		payloads: payloads,
	}

	return rv
}

func (s *sealValidator) verifySealSignature(aggregatedSignatures *flow.AggregatedSignature,
	chunk *flow.Chunk, execurtionResult *flow.ExecutionResult) error {
	for i, signature := range aggregatedSignatures.VerifierSignatures {
		signerId := aggregatedSignatures.SignerIDs[i]

		nodeIdentity, err := identityForNode(s.state, chunk.BlockID, signerId)

		if err != nil {
			return err
		}

		atst := flow.Attestation{
			BlockID:           chunk.BlockID,
			ExecutionResultID: execurtionResult.ID(),
			ChunkIndex:        chunk.Index,
		}
		atstID := atst.ID()
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

func (s *sealValidator) Validate(candidate *flow.Block) (*flow.Seal, error) {
	// Get the latest seal in the fork that ends with the candidate's parent.
	// The protocol state saves this information for each block that has been
	// successfully added to the chain tree (even when the added block does not
	// itself contain a seal). We just called `headerExtend` to check that the
	// candidate block's header is a valid extension of the chain, which implies
	// that the parent must already be part of the chain tree. Therefore, _not_
	// finding the latest sealed block in the fork up to the parent constitutes
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
		return nil, state.NewInvalidExtensionErrorf("multiple seals for the same block")
	}

	// incorporatedResults collects the _first_ appearance of unsealed execution
	// results on the fork, along with the ID of the block in which they are
	// incorporated.
	incorporatedResults := make(map[flow.Identifier]*flow.IncorporatedResult)

	// collect IDs of blocks on the fork (from parent to last sealed)
	var blockIDs []flow.Identifier

	// loop through the fork backwards up to last sealed block and collect
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
			// incorporatedResults should contain the _first_ appearance of the
			// ExecutionResult. We are traversing the fork backwards, so we can
			// overwrite any previously recorded result.

			// ATTENTION:
			// Here, IncorporatedBlockID (the first argument) should be set
			// to ancestorID, because that is the block that contains the
			// ExecutionResult. However, in phase 2 of the sealing roadmap,
			// we are still using a temporary sealing logic where the
			// IncorporatedBlockID is expected to be the result's block ID.
			incorporatedResults[resultID] = flow.NewIncorporatedResult(
				receipt.ExecutionResult.BlockID,
				&receipt.ExecutionResult,
			)

		}

		ancestorID = ancestor.ParentID
	}

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
			return nil, state.NewInvalidExtensionErrorf("chain of seals broken (missing: %x)", blockID)
		}

		delete(byBlock, blockID)

		// check if we have an incorporatedResult for this seal
		incorporatedResult, ok := incorporatedResults[seal.ResultID]
		if !ok {
			return nil, state.NewInvalidExtensionErrorf("seal %x does not correspond to a result on this fork", seal.ID())
		}

		// check the integrity of the seal
		err := s.validateSeal(seal, incorporatedResult.Result)
		if err != nil {
			if engine.IsInvalidInputError(err) {
				return nil, state.NewInvalidExtensionErrorf("payload includes invalid seal (%x), %w", seal.ID(), err)
			}

			return nil, fmt.Errorf("unexpected seal validation error %w", err)
		}

		last = seal
	}

	// at this point no seals should be left
	if len(byBlock) > 0 {
		return nil, state.NewInvalidExtensionErrorf("not all seals connected to state (left: %d)", len(byBlock))
	}

	return last, nil
}

func (s *sealValidator) validateSeal(seal *flow.Seal, executionResult *flow.ExecutionResult) error {
	if executionResult.BlockID != seal.BlockID {
		return engine.NewInvalidInputErrorf("invalid blockID for seal, expected: %v got: %v",
			executionResult.BlockID, seal.BlockID)
	}

	if len(seal.AggregatedApprovalSigs) != executionResult.Chunks.Len() {
		return engine.NewInvalidInputErrorf("mismatching signatures, expected: %d, got: %d",
			executionResult.Chunks.Len(),
			len(seal.AggregatedApprovalSigs))
	}

	assignemnts, err := s.assigner.Assign(executionResult, seal.BlockID)
	if err != nil {
		return fmt.Errorf("could not retreive assignments for block: %v, %w", seal.BlockID, err)
	}

	for _, chunk := range executionResult.Chunks {
		chunkSigs := seal.AggregatedApprovalSigs[chunk.Index]
		assignedVerifiers := assignemnts.Verifiers(chunk)
		lenSignerIds := len(chunkSigs.SignerIDs)
		if lenSignerIds != assignedVerifiers.Len() {
			return engine.NewInvalidInputErrorf("mismatched signature ids length %d vs %d",
				lenSignerIds, assignedVerifiers.Len())
		}

		lenVerifierSigs := len(chunkSigs.VerifierSignatures)
		if lenVerifierSigs != lenSignerIds {
			return engine.NewInvalidInputErrorf("mismatched signatures length %d vs %d",
				lenVerifierSigs, lenSignerIds)
		}

		for _, signerId := range chunkSigs.SignerIDs {
			if !assignemnts.HasVerifier(chunk, signerId) {
				return engine.NewInvalidInputErrorf("invalid signer id at chunk: %d", chunk.Index)
			}
		}

		err := s.verifySealSignature(&chunkSigs, chunk, executionResult)
		if err != nil {
			return fmt.Errorf("invalid seal signature: %w", err)
		}
	}

	return nil
}
