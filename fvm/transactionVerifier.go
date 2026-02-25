package fvm

import (
	"context"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel/attribute"

	"github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

type signatureType struct {
	payload []byte

	errorBuilder func(flow.TransactionSignature, error) errors.CodedError

	aggregateWeights map[flow.Address]int
}

type signatureEntry struct {
	flow.TransactionSignature

	signatureType
}

// signatureContinatuion is an internal/helper struct, accessible only by
// TransactionVerifier, used to keep track of the signature verification
// continuation state.
type signatureContinuation struct {
	// signatureEntry is the initial input.
	signatureEntry

	// accountKey is set by getAccountKeysAndAggregateWeights().
	accountKey flow.RuntimeAccountPublicKey

	// invokedVerify and verifyErr are set by verifySignatures().  Note
	// that	verifySignatures() is always called after getAccountKeysAndAggregateWeights()
	// (i.e., accountKey is always initialized by the time
	// verifySignatures is called).
	invokedVerify bool
	verifyErr     errors.CodedError
}

func (entry *signatureContinuation) newError(err error) errors.CodedError {
	return entry.errorBuilder(entry.TransactionSignature, err)
}

func (entry *signatureContinuation) matches(
	proposalKey flow.ProposalKey,
) bool {
	return entry.Address == proposalKey.Address &&
		entry.KeyIndex == proposalKey.KeyIndex
}

func (entry *signatureContinuation) verify() errors.CodedError {
	if entry.invokedVerify {
		return entry.verifyErr
	}

	entry.invokedVerify = true

	valid, message := entry.ValidateExtensionDataAndReconstructMessage(entry.payload)
	if !valid {
		entry.verifyErr = entry.newError(fmt.Errorf("signature extension data is not valid"))
		return entry.verifyErr
	}

	valid, err := crypto.VerifySignatureFromTransaction(
		entry.Signature,
		message,
		entry.accountKey.PublicKey,
		entry.accountKey.HashAlgo,
	)
	if err != nil {
		entry.verifyErr = entry.newError(err)
	} else if !valid {
		entry.verifyErr = entry.newError(fmt.Errorf("signature is not valid"))
	}

	return entry.verifyErr
}

// newSignatureEntries creates a list of signatureContinuation entries and deduplicate signatures
// per account key.
// The function returns an error if:
// - there are duplicate signatures for the same account key (address and key index pair)
// - a signature is provided for an account that is not a payer, proposer or authorizer
func newSignatureEntries(tx *flow.TransactionBody) (
	[]*signatureContinuation,
	map[flow.Address]int,
	map[flow.Address]int,
	error,
) {
	transactionAddresses := make(map[flow.Address]struct{})
	transactionAddresses[tx.Payer] = struct{}{}
	transactionAddresses[tx.ProposalKey.Address] = struct{}{}
	for _, addr := range tx.Authorizers {
		transactionAddresses[addr] = struct{}{}
	}

	// weight maps are assigned to entries in this function, but are returned as empty maps
	payloadWeights := make(map[flow.Address]int, len(tx.PayloadSignatures))
	envelopeWeights := make(map[flow.Address]int, len(tx.EnvelopeSignatures))

	type pair struct {
		signatureType
		signatures []flow.TransactionSignature
	}

	list := []pair{
		{
			signatureType{
				tx.PayloadMessage(),
				errors.NewInvalidPayloadSignatureError,
				payloadWeights,
			},
			tx.PayloadSignatures,
		},
		{
			signatureType{
				tx.EnvelopeMessage(),
				errors.NewInvalidEnvelopeSignatureError,
				envelopeWeights,
			},
			tx.EnvelopeSignatures,
		},
	}

	numSignatures := len(tx.PayloadSignatures) + len(tx.EnvelopeSignatures)
	signatureContinuations := make([]*signatureContinuation, 0, numSignatures)

	type uniqueKey struct {
		address flow.Address
		index   uint32
	}
	duplicate := make(map[uniqueKey]struct{}, numSignatures)

	for _, group := range list {
		for _, signature := range group.signatures {
			entry := &signatureContinuation{
				signatureEntry: signatureEntry{
					TransactionSignature: signature,
					signatureType:        group.signatureType,
				},
			}

			// check signature address is either payer, proposer or authorizer
			_, ok := transactionAddresses[signature.Address]
			if !ok {
				return nil, nil, nil, entry.newError(
					fmt.Errorf("signature is provided for account %s that is neither payer nor authorizer nor proposer", signature.Address))
			}

			key := uniqueKey{
				address: signature.Address,
				index:   signature.KeyIndex,
			}

			_, ok = duplicate[key]
			if ok {
				return nil, nil, nil, entry.newError(
					fmt.Errorf("duplicate signatures are provided for the same key"))
			}
			duplicate[key] = struct{}{}
			signatureContinuations = append(signatureContinuations, entry)
		}
	}

	return signatureContinuations, payloadWeights, envelopeWeights, nil
}

// TransactionVerifier verifies the content of the transaction by
// checking there is no double signature
// all signatures are valid
// all accounts provides enoguh weights
//
// if KeyWeightThreshold is set to a negative number, signature verification is skipped
type TransactionVerifier struct {
	VerificationConcurrency int
}

func (v *TransactionVerifier) CheckAuthorization(
	tracer tracing.TracerSpan,
	proc *TransactionProcedure,
	txnState storage.TransactionPreparer,
	keyWeightThreshold int,
) error {
	// TODO(Janez): verification is part of inclusion fees, not execution fees.
	var err error
	txnState.RunWithMeteringDisabled(func() {
		err = v.verifyTransaction(tracer, proc, txnState, keyWeightThreshold)
	})
	if err != nil {
		return fmt.Errorf("transaction verification failed: %w", err)
	}

	return nil
}

// verifyTransaction verifies the transaction from the given procedure,
// and check the Authorizers have enough weights.
func (v *TransactionVerifier) verifyTransaction(
	tracer tracing.TracerSpan,
	proc *TransactionProcedure,
	txnState storage.TransactionPreparer,
	keyWeightThreshold int,
) error {
	span := tracer.StartChildSpan(trace.FVMVerifyTransaction)
	span.SetAttributes(
		attribute.String("transaction.ID", proc.ID.String()),
	)
	defer span.End()

	tx := proc.Transaction
	if tx.Payer == flow.EmptyAddress {
		return errors.NewInvalidAddressErrorf(tx.Payer, "payer address is invalid")
	}

	// return the signature entries (both payload and envelope) and empty weight maps
	// that will be used to aggregate weights.
	// the account keys are deduplicated during this call.
	signatures, payloadWeights, envelopeWeights, err := newSignatureEntries(tx)
	if err != nil {
		return err
	}

	accounts := environment.NewAccounts(txnState)

	if keyWeightThreshold < 0 {
		return nil
	}

	// at this point, account keys are guaranteed to be unique across all signatures
	err = v.getAccountKeysAndAggregateWeights(txnState, accounts, signatures, tx.ProposalKey)
	if err != nil {
		return errors.NewInvalidProposalSignatureError(tx.ProposalKey, err)
	}

	// all authorizers must have sufficient weights
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

	// payer must have sufficient weights
	if !v.hasSufficientKeyWeight(envelopeWeights, tx.Payer, keyWeightThreshold) {
		// TODO change this to payer error (needed for fees)
		return errors.NewAccountAuthorizationErrorf(
			tx.Payer,
			"payer account does not have sufficient signatures (%d < %d)",
			envelopeWeights[tx.Payer],
			keyWeightThreshold)
	}

	// Verify all cryptographic signatures against account public keys (concurrently)
	// and fail if at least one signature is invalid.
	// (at this point, signatures have been deduplicated and weights have been checked,
	// we wouldn't verify the signatures if any of those checks failed)
	err = v.verifySignatures(signatures)
	if err != nil {
		return errors.NewInvalidProposalSignatureError(tx.ProposalKey, err)
	}

	return nil
}

// getAccountKeysAndAggregateWeights gets the signatures' account keys and populate the
// keys and their weights into the signature continuation structs.
func (v *TransactionVerifier) getAccountKeysAndAggregateWeights(
	_ storage.TransactionPreparer,
	accounts environment.Accounts,
	signatures []*signatureContinuation,
	proposalKey flow.ProposalKey,
) error {
	foundProposalSignature := false
	for _, signature := range signatures {
		accountKey, err := accounts.GetRuntimeAccountPublicKey(
			signature.Address,
			signature.KeyIndex)
		if err != nil {
			return signature.newError(err)
		}

		if accountKey.Revoked {
			return signature.newError(
				fmt.Errorf("account key has been revoked"))
		}

		signature.accountKey = accountKey
		// aggregateWeight
		signature.aggregateWeights[signature.Address] += accountKey.Weight

		if !foundProposalSignature && signature.matches(proposalKey) {
			foundProposalSignature = true
		}
	}

	if !foundProposalSignature {
		return fmt.Errorf(
			"either the payload or the envelope should provide proposal " +
				"signatures")
	}

	return nil
}

// verifySignatures verifies the given cryptographic signature continuations (concurrently).
// It returns an error if at least one signature is invalid and no error if all signatures are valid.
func (v *TransactionVerifier) verifySignatures(
	signatures []*signatureContinuation,
) error {
	toVerifyChan := make(chan *signatureContinuation, len(signatures))
	verifiedChan := make(chan *signatureContinuation, len(signatures))

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
	for range signatures {
		entry := <-verifiedChan

		if !entry.invokedVerify {
			// This is a programming error.
			return fmt.Errorf("signatureContinuation.verify not called")
		}

		if entry.verifyErr != nil {
			// Unfortunately, we cannot return the first error we received
			// from the verifiedChan since the entries may be out of order,
			// which could lead to non-deterministic error output.
			foundError = true
			break
		}
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
