package fvm

import (
	"fmt"

	"go.opentelemetry.io/otel/attribute"

	"github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/trace"
)

// TransactionVerifier verifies the content of the transaction by
// checking accounts (authorizers, payer, proposer) are not frozen
// checking there is no double signature
// all signatures are valid
// all accounts provides enoguh weights
//
// if KeyWeightThreshold is set to a negative number, signature verification is skipped
type TransactionVerifier struct {
}

func (v *TransactionVerifier) CheckAuthorization(
	tracer module.Tracer,
	proc *TransactionProcedure,
	txnState *state.TransactionState,
	keyWeightThreshold int,
) error {
	// TODO(Janez): verification is part of inclusion fees, not execution fees.
	var err error
	txnState.RunWithAllLimitsDisabled(func() {
		err = v.verifyTransaction(tracer, proc, txnState, keyWeightThreshold)
	})
	if err != nil {
		return fmt.Errorf("transaction verification failed: %w", err)
	}

	return nil
}

func (v *TransactionVerifier) verifyTransaction(
	tracer module.Tracer,
	proc *TransactionProcedure,
	txnState *state.TransactionState,
	keyWeightThreshold int,
) error {
	span := proc.StartSpanFromProcTraceSpan(tracer, trace.FVMVerifyTransaction)
	span.SetAttributes(
		attribute.String("transaction.ID", proc.ID.String()),
	)
	defer span.End()

	tx := proc.Transaction
	accounts := environment.NewAccounts(txnState)
	if tx.Payer == flow.EmptyAddress {
		return errors.NewInvalidAddressErrorf(tx.Payer, "payer address is invalid")
	}

	var err error
	var payloadWeights map[flow.Address]int
	var proposalKeyVerifiedInPayload bool

	err = v.checkSignatureDuplications(tx)
	if err != nil {
		return err
	}

	err = v.checkAccountsAreNotFrozen(tx, accounts)
	if err != nil {
		return err
	}

	if keyWeightThreshold < 0 {
		return nil
	}

	payloadWeights, proposalKeyVerifiedInPayload, err = v.verifyAccountSignatures(
		txnState,
		accounts,
		tx.PayloadSignatures,
		tx.PayloadMessage(),
		tx.ProposalKey,
		errors.NewInvalidPayloadSignatureError,
	)
	if err != nil {
		return errors.NewInvalidProposalSignatureError(tx.ProposalKey, err)
	}

	var envelopeWeights map[flow.Address]int
	var proposalKeyVerifiedInEnvelope bool

	envelopeWeights, proposalKeyVerifiedInEnvelope, err = v.verifyAccountSignatures(
		txnState,
		accounts,
		tx.EnvelopeSignatures,
		tx.EnvelopeMessage(),
		tx.ProposalKey,
		errors.NewInvalidEnvelopeSignatureError,
	)
	if err != nil {
		return errors.NewInvalidProposalSignatureError(tx.ProposalKey, err)

	}

	proposalKeyVerified := proposalKeyVerifiedInPayload || proposalKeyVerifiedInEnvelope
	if !proposalKeyVerified {
		err := fmt.Errorf("either the payload or the envelope should provide proposal signatures")
		return errors.NewInvalidProposalSignatureError(tx.ProposalKey, err)
	}

	for _, addr := range tx.Authorizers {
		// Skip this authorizer if it is also the payer. In the case where an account is
		// both a PAYER as well as an AUTHORIZER or PROPOSER, that account is required
		// to sign only the envelope.
		if addr == tx.Payer {
			continue
		}
		// hasSufficientKeyWeight
		if !v.hasSufficientKeyWeight(payloadWeights, addr, keyWeightThreshold) {
			return errors.NewAccountAuthorizationErrorf(
				addr,
				"authorizer account does not have sufficient signatures (%d < %d)",
				payloadWeights[addr],
				keyWeightThreshold)
		}
	}

	if !v.hasSufficientKeyWeight(envelopeWeights, tx.Payer, keyWeightThreshold) {
		// TODO change this to payer error (needed for fees)
		return errors.NewAccountAuthorizationErrorf(
			tx.Payer,
			"payer account does not have sufficient signatures (%d < %d)",
			envelopeWeights[tx.Payer],
			keyWeightThreshold)
	}

	return nil
}

func (v *TransactionVerifier) verifyAccountSignatures(
	txnState *state.TransactionState,
	accounts environment.Accounts,
	signatures []flow.TransactionSignature,
	message []byte,
	proposalKey flow.ProposalKey,
	errorBuilder func(flow.TransactionSignature, error) errors.CodedError,
) (
	weights map[flow.Address]int,
	proposalKeyVerified bool,
	err error,
) {
	weights = make(map[flow.Address]int)

	for _, txSig := range signatures {

		accountKey, err := accounts.GetPublicKey(txSig.Address, txSig.KeyIndex)
		if err != nil {
			return nil, false, errorBuilder(txSig, err)
		}
		err = v.verifyAccountSignature(accountKey, txSig, message, errorBuilder)
		if err != nil {
			return nil, false, err
		}
		if !proposalKeyVerified && v.sigIsForProposalKey(txSig, proposalKey) {
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
func (v *TransactionVerifier) verifyAccountSignature(
	accountKey flow.AccountPublicKey,
	txSig flow.TransactionSignature,
	message []byte,
	errorBuilder func(flow.TransactionSignature, error) errors.CodedError,
) error {

	if accountKey.Revoked {
		return errorBuilder(txSig, fmt.Errorf("account key has been revoked"))
	}

	valid, err := crypto.VerifySignatureFromTransaction(
		txSig.Signature,
		message,
		accountKey.PublicKey,
		accountKey.HashAlgo,
	)
	if err != nil {
		return errorBuilder(txSig, err)
	}

	if valid {
		return nil
	}

	return errorBuilder(txSig, fmt.Errorf("signature is not valid"))
}

func (v *TransactionVerifier) hasSufficientKeyWeight(
	weights map[flow.Address]int,
	address flow.Address,
	keyWeightThreshold int,
) bool {
	return weights[address] >= keyWeightThreshold
}

func (v *TransactionVerifier) sigIsForProposalKey(
	txSig flow.TransactionSignature,
	proposalKey flow.ProposalKey,
) bool {
	return txSig.Address == proposalKey.Address && txSig.KeyIndex == proposalKey.KeyIndex
}

func (v *TransactionVerifier) checkSignatureDuplications(tx *flow.TransactionBody) error {
	type uniqueKey struct {
		address flow.Address
		index   uint64
	}
	observedSigs := make(map[uniqueKey]bool)
	for _, sig := range tx.PayloadSignatures {
		if observedSigs[uniqueKey{sig.Address, sig.KeyIndex}] {
			return errors.NewInvalidPayloadSignatureError(
				sig,
				fmt.Errorf("duplicate signatures are provided for the same key"))
		}
		observedSigs[uniqueKey{sig.Address, sig.KeyIndex}] = true
	}

	for _, sig := range tx.EnvelopeSignatures {
		if observedSigs[uniqueKey{sig.Address, sig.KeyIndex}] {
			return errors.NewInvalidEnvelopeSignatureError(
				sig,
				fmt.Errorf("duplicate signatures are provided for the same key"))
		}
		observedSigs[uniqueKey{sig.Address, sig.KeyIndex}] = true
	}
	return nil
}

func (v *TransactionVerifier) checkAccountsAreNotFrozen(
	tx *flow.TransactionBody,
	accounts environment.Accounts,
) error {
	authorizers := make([]flow.Address, 0, len(tx.Authorizers)+2)
	authorizers = append(authorizers, tx.Authorizers...)
	authorizers = append(authorizers, tx.ProposalKey.Address, tx.Payer)

	for _, authorizer := range authorizers {
		err := accounts.CheckAccountNotFrozen(authorizer)
		if err != nil {
			return fmt.Errorf("checking frozen account failed: %w", err)
		}
	}

	return nil
}
