package fvm

import (
	"fmt"

	"github.com/opentracing/opentracing-go/log"

	"github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

type signType int

const (
	payloadSignature  signType = 0
	envelopeSignature signType = 1
)

type TransactionSignatureVerifier struct {
	SignatureVerifier  crypto.SignatureVerifier
	KeyWeightThreshold int
}

func NewTransactionSignatureVerifier(keyWeightThreshold int) *TransactionSignatureVerifier {
	return &TransactionSignatureVerifier{
		SignatureVerifier:  crypto.DefaultSignatureVerifier{},
		KeyWeightThreshold: keyWeightThreshold,
	}
}

func (v *TransactionSignatureVerifier) Process(
	_ *VirtualMachine,
	ctx *Context,
	proc *TransactionProcedure,
	sth *state.StateHolder,
	programs *programs.Programs,
) error {
	return v.verifyTransactionSignatures(proc, *ctx, sth)
}

func (v *TransactionSignatureVerifier) verifyTransactionSignatures(
	proc *TransactionProcedure,
	ctx Context,
	sth *state.StateHolder,
) error {
	if ctx.Tracer != nil && proc.TraceSpan != nil {
		span := ctx.Tracer.StartSpanFromParent(proc.TraceSpan, trace.FVMVerifyTransaction)
		span.LogFields(
			log.String("transaction.ID", proc.ID.String()),
		)
		defer span.Finish()
	}

	tx := proc.Transaction
	accounts := state.NewAccounts(sth)
	if tx.Payer == flow.EmptyAddress {
		err := errors.NewInvalidAddressErrorf(tx.Payer, "payer address is invalid")
		return fmt.Errorf("transaction verification failed: %w", err)
	}

	var err error
	var payloadWeights map[flow.Address]int
	var proposalKeyVerifiedInPayload bool

	err = v.checkSignatureDuplications(tx)
	if err != nil {
		return fmt.Errorf("transaction verification failed: %w", err)
	}

	payloadWeights, proposalKeyVerifiedInPayload, err = v.aggregateAccountSignatures(
		accounts,
		tx.PayloadSignatures,
		tx.PayloadMessage(),
		tx.ProposalKey,
		payloadSignature,
	)
	if err != nil {
		return errors.NewInvalidProposalSignatureError(tx.ProposalKey.Address, tx.ProposalKey.KeyIndex, err)
	}

	var envelopeWeights map[flow.Address]int
	var proposalKeyVerifiedInEnvelope bool

	envelopeWeights, proposalKeyVerifiedInEnvelope, err = v.aggregateAccountSignatures(
		accounts,
		tx.EnvelopeSignatures,
		tx.EnvelopeMessage(),
		tx.ProposalKey,
		envelopeSignature,
	)
	if err != nil {
		return errors.NewInvalidProposalSignatureError(tx.ProposalKey.Address, tx.ProposalKey.KeyIndex, err)

	}

	proposalKeyVerified := proposalKeyVerifiedInPayload || proposalKeyVerifiedInEnvelope
	if !proposalKeyVerified {
		err := fmt.Errorf("either the payload or the envelope should provide proposal signatures")
		return errors.NewInvalidProposalSignatureError(tx.ProposalKey.Address, tx.ProposalKey.KeyIndex, err)
	}

	for _, addr := range tx.Authorizers {
		// Skip this authorizer if it is also the payer. In the case where an account is
		// both a PAYER as well as an AUTHORIZER or PROPOSER, that account is required
		// to sign only the envelope.
		if addr == tx.Payer {
			continue
		}
		// hasSufficientKeyWeight
		if !v.hasSufficientKeyWeight(payloadWeights, addr) {
			msg := fmt.Sprintf("authorizer account does not have sufficient signatures (%d < %d)", payloadWeights[addr], v.KeyWeightThreshold)
			return errors.NewAccountAuthorizationErrorf(addr, msg)
		}
	}

	if !v.hasSufficientKeyWeight(envelopeWeights, tx.Payer) {
		// TODO change this to payer error (needed for fees)
		msg := fmt.Sprintf("payer account does not have sufficient signatures (%d < %d)", envelopeWeights[tx.Payer], v.KeyWeightThreshold)
		return errors.NewAccountAuthorizationErrorf(tx.Payer, msg)
	}

	return nil
}

func (v *TransactionSignatureVerifier) aggregateAccountSignatures(
	accounts state.Accounts,
	signatures []flow.TransactionSignature,
	message []byte,
	proposalKey flow.ProposalKey,
	sType signType,
) (
	weights map[flow.Address]int,
	proposalKeyVerified bool,
	err error,
) {
	weights = make(map[flow.Address]int)

	var accountKey *flow.AccountPublicKey
	for _, txSig := range signatures {
		accountKey, err = v.verifyAccountSignature(accounts, txSig, message, sType)
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
	accounts state.Accounts,
	txSig flow.TransactionSignature,
	message []byte,
	sType signType,
) (*flow.AccountPublicKey, error) {
	accountKey, err := accounts.GetPublicKey(txSig.Address, txSig.KeyIndex)
	if err != nil {
		if sType == envelopeSignature {
			return nil, errors.NewInvalidEnvelopeSignatureError(txSig.Address, txSig.KeyIndex, err)
		}
		return nil, errors.NewInvalidPayloadSignatureError(txSig.Address, txSig.KeyIndex, err)
	}

	if accountKey.Revoked {
		err = fmt.Errorf("account key has been revoked")
		if sType == envelopeSignature {
			return nil, errors.NewInvalidEnvelopeSignatureError(txSig.Address, txSig.KeyIndex, err)
		}
		return nil, errors.NewInvalidPayloadSignatureError(txSig.Address, txSig.KeyIndex, err)
	}

	valid, err := v.SignatureVerifier.Verify(
		txSig.Signature,
		string(flow.TransactionDomainTag[:]),
		message,
		accountKey.PublicKey,
		accountKey.HashAlgo,
	)
	if err != nil {
		if sType == envelopeSignature {
			return nil, errors.NewInvalidEnvelopeSignatureError(txSig.Address, txSig.KeyIndex, err)
		}
		return nil, errors.NewInvalidPayloadSignatureError(txSig.Address, txSig.KeyIndex, err)
	}

	if valid {
		return &accountKey, nil
	}

	err = fmt.Errorf("signature is not valid")
	if sType == envelopeSignature {
		return nil, errors.NewInvalidEnvelopeSignatureError(txSig.Address, txSig.KeyIndex, err)
	}
	return nil, errors.NewInvalidPayloadSignatureError(txSig.Address, txSig.KeyIndex, err)
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

func (v *TransactionSignatureVerifier) checkSignatureDuplications(tx *flow.TransactionBody) error {
	observedSigs := make(map[string]bool)
	for _, sig := range tx.PayloadSignatures {
		key := sig.UniqueKeyString()
		if observedSigs[key] {
			err := fmt.Errorf("duplicate signatures are provided for the same key")
			return errors.NewInvalidPayloadSignatureError(sig.Address, sig.KeyIndex, err)
		}
		observedSigs[key] = true
	}

	for _, sig := range tx.EnvelopeSignatures {
		key := sig.UniqueKeyString()
		if observedSigs[key] {
			err := fmt.Errorf("duplicate signatures are provided for the same key")
			return errors.NewInvalidEnvelopeSignatureError(sig.Address, sig.KeyIndex, err)
		}
		observedSigs[key] = true
	}
	return nil
}
