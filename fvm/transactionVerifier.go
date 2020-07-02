package fvm

import (
	"errors"

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
	ledger Ledger,
) error {
	return v.verifyTransactionSignatures(proc.Transaction, ledger)
}

func (v *TransactionSignatureVerifier) verifyTransactionSignatures(
	tx *flow.TransactionBody,
	ledger Ledger,
) (err error) {
	if tx.Payer == flow.EmptyAddress {
		return &MissingPayerError{}
	}

	var payloadWeights map[flow.Address]int
	var proposalKeyVerifiedInPayload bool

	payloadWeights, proposalKeyVerifiedInPayload, err = v.aggregateAccountSignatures(
		ledger,
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
		ledger,
		tx.EnvelopeSignatures,
		tx.EnvelopeMessage(),
		tx.ProposalKey,
	)
	if err != nil {
		return err
	}

	proposalKeyVerified := proposalKeyVerifiedInPayload || proposalKeyVerifiedInEnvelope

	if !proposalKeyVerified {
		return &MissingSignatureForProposalKeyError{
			Address: tx.ProposalKey.Address,
			KeyID:   tx.ProposalKey.KeyID,
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
	ledger Ledger,
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
		accountKey, valid, err := v.verifyAccountSignature(ledger, txSig, message)
		if err != nil {
			return nil, false, err
		}

		if !valid {
			return nil, false, &InvalidSignaturePublicKeyError{
				Address: txSig.Address,
				KeyID:   txSig.KeyID,
			}
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
	ledger Ledger,
	txSig flow.TransactionSignature,
	message []byte,
) (*flow.AccountPublicKey, bool, error) {
	var ok bool
	var err error

	ok, err = accountExists(ledger, txSig.Address)
	if err != nil {
		return nil, false, err
	}

	if !ok {
		return nil, false, &InvalidSignatureAccountError{Address: txSig.Address}
	}

	var accountKeys []flow.AccountPublicKey

	accountKeys, err = getAccountPublicKeys(ledger, txSig.Address)
	if err != nil {
		return nil, false, err
	}

	if int(txSig.KeyID) >= len(accountKeys) {
		return nil, false, &InvalidSignatureAccountError{Address: txSig.Address}
	}

	accountKey := &accountKeys[txSig.KeyID]

	valid, err := v.SignatureVerifier.Verify(
		txSig.Signature,
		nil, // TODO: include transaction signature tag
		message,
		accountKey.PublicKey,
		accountKey.HashAlgo,
	)
	if err != nil {
		if errors.Is(err, ErrInvalidHashAlgorithm) {
			return nil, false, &InvalidHashAlgorithmError{
				Address:  txSig.Address,
				KeyID:    txSig.KeyID,
				HashAlgo: accountKey.HashAlgo,
			}
		}

		return nil, false, &PublicKeyVerificationError{
			Address: txSig.Address,
			KeyID:   txSig.KeyID,
			Err:     err,
		}
	}

	return accountKey, valid, nil
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
