package fvm

import (
	"fmt"

	"github.com/opentracing/opentracing-go/log"

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
		return &errors.InvalidAddressError{Address: tx.Payer, Err: fmt.Errorf("payer address is invalid")}
	}

	var err error
	var payloadWeights map[flow.Address]int
	var proposalKeyVerifiedInPayload bool

	payloadWeights, proposalKeyVerifiedInPayload, err = v.aggregateAccountSignatures(
		accounts,
		tx.PayloadSignatures,
		tx.PayloadMessage(),
		tx.ProposalKey,
		payloadSignature,
	)
	if err != nil {
		return fmt.Errorf("verifying transaction signatures failed: %w", err)
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
		return fmt.Errorf("verifying transaction signatures failed: %w", err)
	}

	proposalKeyVerified := proposalKeyVerifiedInPayload || proposalKeyVerifiedInEnvelope
	if !proposalKeyVerified {
		return &errors.InvalidProposalSignatureError{
			Address:  tx.ProposalKey.Address,
			KeyIndex: tx.ProposalKey.KeyIndex,
			Err:      fmt.Errorf("either the payload or the envelope should provide proposal signatures"),
		}
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
			issue := fmt.Errorf("authorizer account does not have sufficient signatures (got: %d)", payloadWeights[addr])
			return &errors.AuthorizationError{Address: addr, Err: issue}
		}
	}

	if !v.hasSufficientKeyWeight(envelopeWeights, tx.Payer) {
		// TODO change this to payer error (needed for fees)
		issue := fmt.Errorf("payer account does not have sufficient signatures (got: %d)", envelopeWeights[tx.Payer])
		return &errors.AuthorizationError{Address: tx.Payer, Err: issue}
	}

	return nil
}

func (v *TransactionSignatureVerifier) aggregateAccountSignatures(
	accounts *state.Accounts,
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
	accounts *state.Accounts,
	txSig flow.TransactionSignature,
	message []byte,
	sType signType,
) (*flow.AccountPublicKey, error) {
	accountKey, err := accounts.GetPublicKey(txSig.Address, txSig.KeyIndex)
	if err != nil {
		if sType == envelopeSignature {
			return nil,
				&errors.InvalidEnvelopeSignatureError{
					Address:  txSig.Address,
					KeyIndex: txSig.KeyIndex,
					Err:      err,
				}
		}
		return nil,
			&errors.InvalidPayloadSignatureError{
				Address:  txSig.Address,
				KeyIndex: txSig.KeyIndex,
				Err:      err,
			}
	}

	if accountKey.Revoked {
		if sType == envelopeSignature {
			return nil,
				&errors.InvalidEnvelopeSignatureError{
					Address:  txSig.Address,
					KeyIndex: txSig.KeyIndex,
					Err:      fmt.Errorf("account key has been revoked"),
				}
		}

		return nil, &errors.InvalidPayloadSignatureError{
			Address:  txSig.Address,
			KeyIndex: txSig.KeyIndex,
			Err:      fmt.Errorf("account key has been revoked"),
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
		if sType == envelopeSignature {
			return nil,
				&errors.InvalidEnvelopeSignatureError{
					Address:  txSig.Address,
					KeyIndex: txSig.KeyIndex,
					Err:      err,
				}
		}
		return nil,
			&errors.InvalidPayloadSignatureError{
				Address:  txSig.Address,
				KeyIndex: txSig.KeyIndex,
				Err:      err,
			}
	}

	if !valid {
		if sType == envelopeSignature {
			return nil,
				&errors.InvalidEnvelopeSignatureError{
					Address:  txSig.Address,
					KeyIndex: txSig.KeyIndex,
					Err:      fmt.Errorf("signature is not valid"),
				}
		}
		return nil,
			&errors.InvalidPayloadSignatureError{
				Address:  txSig.Address,
				KeyIndex: txSig.KeyIndex,
				Err:      fmt.Errorf("signature is not valid"),
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
