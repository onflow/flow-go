package fvm

import (
	"bytes"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/base64"
	"fmt"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/onflow/flow-go/crypto/hash"
	"go.opentelemetry.io/otel/attribute"
	"math/big"

	crypto2 "github.com/onflow/flow-go/crypto"

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

	signature := txSig.Signature

	// create io reader from signature bytes
	sigReader := bytes.NewReader(txSig.Signature)
	resp, err := protocol.ParseCredentialRequestResponseBody(sigReader)
	if err == nil {

		expectedChallengeRaw := append(flow.TransactionDomainTag[:], message...)
		expectedChallengeRawHashed := sha256.Sum256(expectedChallengeRaw)
		expectedChallenge := base64.RawURLEncoding.EncodeToString(expectedChallengeRawHashed[:])
		if resp.Response.CollectedClientData.Challenge != expectedChallenge {
			return errorBuilder(txSig, fmt.Errorf("signature is not valid, challenge does not match"))
		}

		clientDataHash := sha256.Sum256(resp.Raw.AssertionResponse.ClientDataJSON)
		sigData := append(resp.Raw.AssertionResponse.AuthenticatorData, clientDataHash[:]...)

		message = sigData
		signature = resp.Raw.AssertionResponse.Signature

		hasher, err := crypto.NewPrefixedHashing(hash.SHA2_256, "")
		if err != nil {
			panic(err)
		}

		type ECDSASignature struct {
			R, S *big.Int
		}

		e := &ECDSASignature{}
		_, err = asn1.Unmarshal(signature, e)
		if err != nil {
			return errorBuilder(txSig, fmt.Errorf("signature is not valid, could not unmarshal signature"))
		}
		rBytes := e.R.Bytes()
		sBytes := e.S.Bytes()
		Nlen := 32
		signature := make([]byte, 2*Nlen)
		// pad the signature with zeroes
		copy(signature[Nlen-len(rBytes):], rBytes)
		copy(signature[2*Nlen-len(sBytes):], sBytes)

		valid, err := accountKey.PublicKey.Verify(signature, message, hasher)
		if err != nil {
			// All inputs are guaranteed to be valid at this stage.
			// The check for crypto.InvalidInputs is only a sanity check
			if crypto2.IsInvalidInputsError(err) {
				return errorBuilder(txSig, err)
			}
			// unexpected error in normal operations
			panic(fmt.Errorf("verify transaction signature failed with unexpected error %w", err))
		}
		if valid {
			return nil
		}

		return errorBuilder(txSig, fmt.Errorf("signature is not valid"))
	}

	valid, err := crypto.VerifySignatureFromTransaction(
		signature,
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

type NoOPHasher struct {
}

func (h NoOPHasher) Algorithm() hash.HashingAlgorithm {
	//TODO implement me
	panic("implement me")
}

func (h NoOPHasher) Size() int {
	//TODO implement me
	panic("implement me")
}

func (h NoOPHasher) ComputeHash(i []byte) hash.Hash {
	return i
}

func (h NoOPHasher) Write(p []byte) (n int, err error) {
	//TODO implement me
	panic("implement me")
}

func (h NoOPHasher) SumHash() hash.Hash {
	//TODO implement me
	panic("implement me")
}

func (h NoOPHasher) Reset() {
	//TODO implement me
	panic("implement me")
}

var _ hash.Hasher = (*NoOPHasher)(nil)

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
