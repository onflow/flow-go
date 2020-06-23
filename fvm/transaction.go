package fvm

import (
	"github.com/onflow/cadence/runtime"

	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/model/flow"
)

func Transaction(tx *flow.TransactionBody) InvokableTransaction {
	return InvokableTransaction{tx: tx}
}

type InvokableTransaction struct {
	tx *flow.TransactionBody
}

func (i InvokableTransaction) Transaction() *flow.TransactionBody {
	return i.tx
}

func (i InvokableTransaction) Parse(ctx Context, ledger Ledger) (Invokable, error) {
	panic("implement me")
}

func (i InvokableTransaction) Invoke(ctx Context, ledger Ledger) (*InvocationResult, error) {
	metaCtx := ctx.NewChild(
		WithSignatureVerification(false),
		WithFeePayments(false),
	)

	txID := i.tx.ID()

	if ctx.Options().signatureVerificationEnabled {
		flowErr, err := verifySignatures(ledger, i.tx)
		if err != nil {
			return nil, err
		}
		if flowErr != nil {
			return &InvocationResult{
				ID:    txID,
				Error: flowErr,
			}, nil
		}

		flowErr, err = checkAndIncrementSequenceNumber(ledger, i.tx.ProposalKey)
		if err != nil {
			return nil, err
		}
		if flowErr != nil {
			return &InvocationResult{
				ID:    txID,
				Error: flowErr,
			}, nil
		}
	}

	if ctx.Options().feePaymentsEnabled {
		result, err := metaCtx.Invoke(deductTransactionFeeTransaction(i.tx.Payer), ledger)
		if err != nil {
			return nil, err
		}

		if result.Error != nil {
			return &InvocationResult{
				ID:    txID,
				Error: result.Error,
			}, nil
		}
	}

	env := newEnvironment(ledger, ctx.Options()).setTransaction(i.tx, metaCtx)

	location := runtime.TransactionLocation(txID[:])

	err := ctx.Runtime().ExecuteTransaction(i.tx.Script, i.tx.Arguments, env, location)

	return createInvocationResult(txID, nil, env.getEvents(), env.getLogs(), err)
}

func checkAndIncrementSequenceNumber(ledger Ledger, proposalKey flow.ProposalKey) (FlowError, error) {
	accountKeys, err := getAccountPublicKeys(ledger, proposalKey.Address)
	if err != nil {
		return nil, err
	}

	if int(proposalKey.KeyID) >= len(accountKeys) {
		return &InvalidProposalKeyError{
			Address: proposalKey.Address,
			KeyID:   proposalKey.KeyID,
		}, nil
	}

	accountKey := accountKeys[proposalKey.KeyID]

	valid := accountKey.SeqNumber == proposalKey.SequenceNumber

	if !valid {
		return &InvalidProposalSequenceNumberError{
			Address:           proposalKey.Address,
			KeyID:             proposalKey.KeyID,
			CurrentSeqNumber:  accountKey.SeqNumber,
			ProvidedSeqNumber: proposalKey.SequenceNumber,
		}, nil
	}

	accountKey.SeqNumber++

	updatedAccountKeyBytes, err := flow.EncodeAccountPublicKey(accountKey)
	if err != nil {
		return nil, err
	}

	setAccountPublicKey(ledger, proposalKey.Address, proposalKey.KeyID, updatedAccountKeyBytes)

	return nil, nil
}

// verifySignatures verifies that a transaction contains the necessary signatures.
//
// An error is returned if any of the expected signatures are invalid or missing.
func verifySignatures(ledger Ledger, tx *flow.TransactionBody) (flowErr FlowError, err error) {
	if tx.Payer == flow.EmptyAddress {
		return &MissingPayerError{}, nil
	}

	var payloadWeights map[flow.Address]int
	var proposalKeyVerifiedInPayload bool

	payloadWeights, proposalKeyVerifiedInPayload, flowErr, err = aggregateAccountSignatures(
		ledger,
		tx.PayloadSignatures,
		tx.PayloadMessage(),
		tx.ProposalKey,
	)
	if err != nil {
		return nil, err
	}

	if flowErr != nil {
		return flowErr, nil
	}

	var envelopeWeights map[flow.Address]int
	var proposalKeyVerifiedInEnvelope bool

	envelopeWeights, proposalKeyVerifiedInEnvelope, flowErr, err = aggregateAccountSignatures(
		ledger,
		tx.EnvelopeSignatures,
		tx.EnvelopeMessage(),
		tx.ProposalKey,
	)
	if err != nil {
		return nil, err
	}

	if flowErr != nil {
		return flowErr, nil
	}

	proposalKeyVerified := proposalKeyVerifiedInPayload || proposalKeyVerifiedInEnvelope

	if !proposalKeyVerified {
		return &MissingSignatureForProposalKeyError{
			Address: tx.ProposalKey.Address,
			KeyID:   tx.ProposalKey.KeyID,
		}, nil
	}

	for _, addr := range tx.Authorizers {
		// Skip this authorizer if it is also the payer. In the case where an account is
		// both a PAYER as well as an AUTHORIZER or PROPOSER, that account is required
		// to sign only the envelope.
		if addr == tx.Payer {
			continue
		}

		if !hasSufficientKeyWeight(payloadWeights, addr) {
			return &MissingSignatureError{addr}, nil
		}
	}

	if !hasSufficientKeyWeight(envelopeWeights, tx.Payer) {
		return &MissingSignatureError{tx.Payer}, nil
	}

	return nil, nil
}

func aggregateAccountSignatures(
	ledger Ledger,
	signatures []flow.TransactionSignature,
	message []byte,
	proposalKey flow.ProposalKey,
) (
	weights map[flow.Address]int,
	proposalKeyVerified bool,
	flowErr FlowError,
	err error,
) {
	weights = make(map[flow.Address]int)

	for _, txSig := range signatures {
		accountKey, flowErr, err := verifyAccountSignature(ledger, txSig, message)
		if err != nil {
			return nil, false, nil, err
		}

		if flowErr != nil {
			return nil, false, flowErr, nil
		}

		if sigIsForProposalKey(txSig, proposalKey) {
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
func verifyAccountSignature(
	ledger Ledger,
	txSig flow.TransactionSignature,
	message []byte,
) (*flow.AccountPublicKey, FlowError, error) {
	var ok bool
	var err error

	ok, err = accountExists(ledger, txSig.Address)
	if err != nil {
		return nil, nil, err
	}

	if !ok {
		return nil, &InvalidSignatureAccountError{Address: txSig.Address}, nil
	}

	var accountKeys []flow.AccountPublicKey

	accountKeys, err = getAccountPublicKeys(ledger, txSig.Address)
	if err != nil {
		return nil, nil, err
	}

	if int(txSig.KeyID) >= len(accountKeys) {
		return nil, &InvalidSignatureAccountError{Address: txSig.Address}, nil
	}

	accountKey := &accountKeys[txSig.KeyID]

	hasher, err := hash.NewHasher(accountKey.HashAlgo)
	if err != nil {
		return accountKey, &InvalidHashingAlgorithmError{
			Address:          txSig.Address,
			KeyID:            txSig.KeyID,
			HashingAlgorithm: accountKey.HashAlgo,
		}, nil
	}

	valid, err := accountKey.PublicKey.Verify(txSig.Signature, message, hasher)
	if err != nil {
		return accountKey, &PublicKeyVerificationError{
			Address: txSig.Address,
			KeyID:   txSig.KeyID,
			Err:     err,
		}, nil
	}

	if !valid {
		return accountKey, &InvalidSignaturePublicKeyError{Address: txSig.Address, KeyID: txSig.KeyID}, nil
	}

	return accountKey, nil, nil
}

func sigIsForProposalKey(txSig flow.TransactionSignature, proposalKey flow.ProposalKey) bool {
	return txSig.Address == proposalKey.Address && txSig.KeyID == proposalKey.KeyID
}

func hasSufficientKeyWeight(weights map[flow.Address]int, address flow.Address) bool {
	return weights[address] >= AccountKeyWeightThreshold
}
