package fvm

import (
	"errors"

	"github.com/opentracing/opentracing-go/log"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

type TransactionSignatureVerifier struct {
	SignatureVerifier  SignatureVerifier
	KeyWeightThreshold int
}

func NewTransactionSignatureVerifier(keyWeightThreshold int) *TransactionSignatureVerifier {
	return &TransactionSignatureVerifier{
		SignatureVerifier:  DefaultSignatureVerifier{},
		KeyWeightThreshold: keyWeightThreshold,
	}
}

func (v *TransactionSignatureVerifier) Process(
	vm *VirtualMachine,
	ctx Context,
	proc *TransactionProcedure,
	stm *state.StateManager,
	programs *Programs,
) error {
	return v.verifyTransactionSignatures(proc, ctx, stm)
}

func (v *TransactionSignatureVerifier) verifyTransactionSignatures(
	proc *TransactionProcedure,
	ctx Context,
	stm *state.StateManager,
) (err error) {

	if ctx.Tracer != nil && proc.TraceSpan != nil {
		span := ctx.Tracer.StartSpanFromParent(proc.TraceSpan, trace.FVMVerifyTransaction)
		span.LogFields(
			log.String("transaction.ID", proc.ID.String()),
		)
		defer span.Finish()
	}

	// TODO: separate payer and proposer errors out

	tx := proc.Transaction

	stm.Nest()
	accounts := state.NewAccounts(stm)
	if tx.Payer == flow.EmptyAddress {
		return &MissingPayerError{}
	}

	var payloadWeights map[flow.Address]int
	var proposalKeyVerifiedInPayload bool

	payloadWeights, proposalKeyVerifiedInPayload, err = v.aggregateAccountSignatures(
		accounts,
		tx.PayloadSignatures,
		tx.PayloadMessage(),
		tx.ProposalKey,
	)
	if err != nil {
		return err
	}

	var envelopeWeights map[flow.Address]int
	var proposalKeyVerifiedInEnvelope bool

	envelopeWeights, proposalKeyVerifiedInEnvelope, err = v.aggregateAccountSignatures(
		accounts,
		tx.EnvelopeSignatures,
		tx.EnvelopeMessage(),
		tx.ProposalKey,
	)
	if err != nil {
		return err
	}

	proposalKeyVerified := proposalKeyVerifiedInPayload || proposalKeyVerifiedInEnvelope

	if !proposalKeyVerified {
		return &InvalidProposalKeyMissingSignatureError{
			Address:  tx.ProposalKey.Address,
			KeyIndex: tx.ProposalKey.KeyIndex,
		}
	}

	for _, addr := range tx.Authorizers {
		// Skip this authorizer if it is also the payer. In the case where an account is
		// both a PAYER as well as an AUTHORIZER or PROPOSER, that account is required
		// to sign only the envelope.
		if addr == tx.Payer {
			continue
		}

		if !v.hasSufficientKeyWeight(payloadWeights, addr) {
			return &MissingSignatureError{addr}
		}
	}

	if !v.hasSufficientKeyWeight(envelopeWeights, tx.Payer) {
		return &MissingSignatureError{tx.Payer}
	}

	return stm.RollUpWithMerge()
}

func (v *TransactionSignatureVerifier) aggregateAccountSignatures(
	accounts *state.Accounts,
	signatures []flow.TransactionSignature,
	message []byte,
	proposalKey flow.ProposalKey,
) (
	weights map[flow.Address]int,
	proposalKeyVerified bool,
	err error,
) {
	weights = make(map[flow.Address]int)

	for _, txSig := range signatures {
		accountKey, err := v.verifyAccountSignature(accounts, txSig, message)
		if err != nil {
			return nil, false, err
		}

		if v.sigIsForProposalKey(txSig, proposalKey) {
			proposalKeyVerified = true
		}

		weights[txSig.Address] += accountKey.Weight
	}

	return
}

// verifyAccountSignature verifies that an account signature is valid for the
// account and given message.
//
// If the signature is valid, this function returns the associated account key.
//
// An error is returned if the account does not contain a public key that
// correctly verifies the signature against the given message.
func (v *TransactionSignatureVerifier) verifyAccountSignature(
	accounts *state.Accounts,
	txSig flow.TransactionSignature,
	message []byte,
) (*flow.AccountPublicKey, error) {
	accountKey, err := accounts.GetPublicKey(txSig.Address, txSig.KeyIndex)
	if err != nil {
		if errors.Is(err, state.ErrAccountPublicKeyNotFound) {
			return nil, &InvalidSignaturePublicKeyDoesNotExistError{
				Address:  txSig.Address,
				KeyIndex: txSig.KeyIndex,
			}
		}

		return nil, err
	}

	if accountKey.Revoked {
		return nil, &InvalidSignaturePublicKeyRevokedError{
			Address:  txSig.Address,
			KeyIndex: txSig.KeyIndex,
		}
	}

	valid, err := v.SignatureVerifier.Verify(
		txSig.Signature,
		nil, // TODO: include transaction signature tag
		message,
		accountKey.PublicKey,
		accountKey.HashAlgo,
	)
	if err != nil {
		if errors.Is(err, ErrInvalidHashAlgorithm) {
			return nil, &InvalidHashAlgorithmError{
				Address:  txSig.Address,
				KeyIndex: txSig.KeyIndex,
				HashAlgo: accountKey.HashAlgo,
			}
		}

		return nil, err
	}

	if !valid {
		return nil, &InvalidSignatureVerificationError{
			Address:  txSig.Address,
			KeyIndex: txSig.KeyIndex,
		}
	}

	return &accountKey, nil
}

func (v *TransactionSignatureVerifier) hasSufficientKeyWeight(
	weights map[flow.Address]int,
	address flow.Address,
) bool {
	return weights[address] >= v.KeyWeightThreshold
}

func (v *TransactionSignatureVerifier) sigIsForProposalKey(
	txSig flow.TransactionSignature,
	proposalKey flow.ProposalKey,
) bool {
	return txSig.Address == proposalKey.Address && txSig.KeyIndex == proposalKey.KeyIndex
}
