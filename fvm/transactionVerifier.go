package fvm

import (
	"errors"

	"github.com/dapperlabs/flow-go/fvm/state"
	"github.com/dapperlabs/flow-go/model/flow"
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
	ledger state.Ledger,
) error {
	accounts := state.NewAccounts(ledger, ctx.Chain)

	return v.verifyTransactionSignatures(proc.Transaction, accounts)
}

func (v *TransactionSignatureVerifier) verifyTransactionSignatures(
	tx *flow.TransactionBody,
	accounts *state.Accounts,
) (err error) {
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
			KeyIndex: tx.ProposalKey.KeyID,
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

	return nil
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
	accountKey, err := accounts.GetPublicKey(txSig.Address, txSig.KeyID)
	if err != nil {
		if errors.Is(err, state.ErrAccountPublicKeyNotFound) {
			return nil, &InvalidSignaturePublicKeyDoesNotExistError{
				Address:  txSig.Address,
				KeyIndex: txSig.KeyID,
			}
		}

		return nil, err
	}

	if accountKey.Revoked {
		return nil, &InvalidSignaturePublicKeyRevokedError{
			Address:  txSig.Address,
			KeyIndex: txSig.KeyID,
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
				KeyIndex: txSig.KeyID,
				HashAlgo: accountKey.HashAlgo,
			}
		}

		return nil, err
	}

	if !valid {
		return nil, &InvalidSignatureVerificationError{
			Address:  txSig.Address,
			KeyIndex: txSig.KeyID,
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
	return txSig.Address == proposalKey.Address && txSig.KeyID == proposalKey.KeyID
}
