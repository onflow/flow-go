package fvm

import (
	"context"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel/attribute"

	"github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/trace"
)

type signatureType struct {
	message []byte

	errorBuilder func(flow.TransactionSignature, error) errors.CodedError

	aggregateWeights map[flow.Address]int
}

type signatureEntryData struct {
	flow.TransactionSignature

	signatureType
}

type signatureEntry struct {
	signatureEntryData

	accountKey flow.AccountPublicKey

	verified bool

	verifyErr errors.CodedError
}

func (entry *signatureEntryData) newError(err error) errors.CodedError {
	return entry.errorBuilder(entry.TransactionSignature, err)
}

func (entry *signatureEntry) matches(
	proposalKey flow.ProposalKey,
) bool {
	return entry.Address == proposalKey.Address &&
		entry.KeyIndex == proposalKey.KeyIndex
}

func (entry *signatureEntry) verify() errors.CodedError {
	if entry.verified {
		return entry.verifyErr
	}

	valid, err := crypto.VerifySignatureFromTransaction(
		entry.Signature,
		entry.message,
		entry.accountKey.PublicKey,
		entry.accountKey.HashAlgo,
	)

	if err != nil {
		entry.verifyErr = entry.newError(err)
	} else if !valid {
		entry.verifyErr = entry.newError(fmt.Errorf("signature is not valid"))
	}

	entry.verified = true

	return entry.verifyErr
}

func newSignatureEntries(
	payloadSignatures []flow.TransactionSignature,
	payloadMessage []byte,
	envelopeSignatures []flow.TransactionSignature,
	envelopeMessage []byte,
) (
	[]*signatureEntryData,
	map[flow.Address]int,
	map[flow.Address]int,
	error,
) {
	payloadWeights := make(map[flow.Address]int, len(payloadSignatures))
	envelopeWeights := make(map[flow.Address]int, len(envelopeSignatures))

	type pair struct {
		signatureType
		signatures []flow.TransactionSignature
	}

	list := []pair{
		{
			signatureType{
				payloadMessage,
				errors.NewInvalidPayloadSignatureError,
				payloadWeights,
			},
			payloadSignatures,
		},
		{
			signatureType{
				envelopeMessage,
				errors.NewInvalidEnvelopeSignatureError,
				envelopeWeights,
			},
			envelopeSignatures,
		},
	}

	numSignatures := len(payloadSignatures) + len(envelopeSignatures)
	signatures := make([]*signatureEntryData, 0, numSignatures)

	type uniqueKey struct {
		address flow.Address
		index   uint64
	}
	duplicate := make(map[uniqueKey]struct{}, numSignatures)

	for _, group := range list {
		for _, signature := range group.signatures {
			entryData := &signatureEntryData{
				TransactionSignature: signature,
				signatureType:        group.signatureType,
			}

			key := uniqueKey{
				address: signature.Address,
				index:   signature.KeyIndex,
			}

			_, ok := duplicate[key]
			if ok {
				return nil, nil, nil, entryData.newError(
					fmt.Errorf("duplicate signatures are provided for the same key"))
			}
			duplicate[key] = struct{}{}
			signatures = append(signatures, entryData)
		}
	}

	return signatures, payloadWeights, envelopeWeights, nil
}

// TransactionVerifier verifies the content of the transaction by
// checking accounts (authorizers, payer, proposer) are not frozen
// checking there is no double signature
// all signatures are valid
// all accounts provides enoguh weights
//
// if KeyWeightThreshold is set to a negative number, signature verification is skipped
type TransactionVerifier struct {
	VerificationConcurrency int
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

// verifyTransaction verifies the transaction from the given procedure,
// and check the Authorizers have enough weights
// it returns:
// - nil, if all checks are passed
// - errors.CodedError, if the transaction did pass certain check
// - other error would be exceptions
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
	if tx.Payer == flow.EmptyAddress {
		return errors.NewInvalidAddressErrorf(tx.Payer, "payer address is invalid")
	}

	signaturesData, payloadWeights, envelopeWeights, err := newSignatureEntries(
		tx.PayloadSignatures,
		tx.PayloadMessage(),
		tx.EnvelopeSignatures,
		tx.EnvelopeMessage())
	if err != nil {
		return err
	}

	accounts := environment.NewAccounts(txnState)
	err = v.checkAccountsAreNotFrozen(tx, accounts)
	if err != nil {
		return err
	}

	if keyWeightThreshold < 0 {
		return nil
	}

	signatures, err := v.getAccountKeys(txnState, accounts, signaturesData, tx.ProposalKey)
	if err != nil {
		return errors.NewInvalidProposalSignatureError(tx.ProposalKey, err)
	}

	err = v.verifyAccountSignatures(signatures)
	if err != nil {
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

func (v *TransactionVerifier) getAccountKeys(
	txnState *state.TransactionState,
	accounts environment.Accounts,
	signaturesData []*signatureEntryData,
	proposalKey flow.ProposalKey,
) ([]*signatureEntry, error) {
	foundProposalSignature := false
	signatures := make([]*signatureEntry, 0, len(signaturesData))
	for _, signatureData := range signaturesData {
		accountKey, err := accounts.GetPublicKey(
			signatureData.Address,
			signatureData.KeyIndex)
		if err != nil {
			return nil, signatureData.newError(err)
		}

		if accountKey.Revoked {
			return nil, signatureData.newError(
				fmt.Errorf("account key has been revoked"))
		}

		signature := signatureEntry{
			signatureEntryData: *signatureData,
			accountKey:         accountKey,
			verified:           false,
			verifyErr:          nil,
		}

		signatures = append(signatures, &signature)

		if !foundProposalSignature && signature.matches(proposalKey) {
			foundProposalSignature = true
		}
	}

	if !foundProposalSignature {
		return nil, fmt.Errorf(
			"either the payload or the envelope should provide proposal " +
				"signatures")
	}

	return signatures, nil
}

// verifyAccountSignatures verifies the given signatures concurrently, and returns
//   - nil, if all signatures are valid
//   - errors.CodedError if at least one signature is invalid, the returned error is
//     for the first invalid signature in order to ensure output to be deterministic
//   - other error would be exceptions
func (v *TransactionVerifier) verifyAccountSignatures(
	signatures []*signatureEntry,
) error {
	toVerifyChan := make(chan *signatureEntry, len(signatures))
	verifiedChan := make(chan *signatureEntry, len(signatures))

	verificationConcurrency := v.VerificationConcurrency
	if len(signatures) < verificationConcurrency {
		verificationConcurrency = len(signatures)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg := sync.WaitGroup{}
	wg.Add(verificationConcurrency)

	for i := 0; i < verificationConcurrency; i++ {
		go func() {
			defer wg.Done()

			for entry := range toVerifyChan {
				err := entry.verify()

				verifiedChan <- entry

				if err != nil {
					// Signal to other workers to early exit
					cancel()
					return
				}

				select {
				case <-ctx.Done():
					// Another worker has error-ed out.
					return
				default:
					// continue
				}
			}
		}()
	}

	for _, entry := range signatures {
		toVerifyChan <- entry
	}
	close(toVerifyChan)

	foundError := false
	for i := 0; i < len(signatures); i++ {
		entry := <-verifiedChan
		if !entry.verified {
			// this would be a software bug
			return fmt.Errorf("fatal error: expected the signature to be verified, but not")
		}

		if entry.verifyErr != nil {
			// Unfortunately, we cannot return the first error we received
			// from the verifiedChan since the entries may be out of order,
			// which could lead to non-deterministic error output.
			foundError = true
			break
		}

		entry.aggregateWeights[entry.Address] += entry.accountKey.Weight
	}

	if !foundError {
		return nil
	}

	// We need to wait for all workers to finish in order to deterministically
	// return the first error with respect to the signatures slice.

	wg.Wait()

	for _, entry := range signatures {
		if entry.verifyErr != nil {
			return entry.verifyErr
		}
	}

	panic("Should never reach here")
}

func (v *TransactionVerifier) hasSufficientKeyWeight(
	weights map[flow.Address]int,
	address flow.Address,
	keyWeightThreshold int,
) bool {
	return weights[address] >= keyWeightThreshold
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
