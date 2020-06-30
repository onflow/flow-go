package fvm

import (
	"errors"

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

func (i InvokableTransaction) Parse(vm *VirtualMachine, ctx Context, ledger Ledger) (Invokable, error) {
	panic("implement me")
}

func (i InvokableTransaction) Invoke(vm *VirtualMachine, ctx Context, ledger Ledger) (*InvocationResult, error) {
	metaCtx := NewContextFromParent(
		ctx,
		WithSignatureVerification(false),
		WithFeePayments(false),
	)

	txID := i.tx.ID()

	if ctx.SignatureVerificationEnabled {
		err := verifySignatures(ledger, i.tx)
		if err != nil {
			return createInvocationResult(txID, nil, nil, nil, err)
		}

		err = checkAndIncrementSequenceNumber(ledger, i.tx.ProposalKey)
		if err != nil {
			return createInvocationResult(txID, nil, nil, nil, err)
		}
	}

	if ctx.FeePaymentsEnabled {
		err := vm.invokeMetaTransaction(
			metaCtx,
			deductTransactionFeeTransaction(i.tx.Payer, vm.chain.ServiceAddress()),
			ledger,
		)
		if err != nil {
			return createInvocationResult(txID, nil, nil, nil, err)
		}
	}

	env := newEnvironment(vm, ctx, ledger).setTransaction(i.tx, metaCtx)

	location := runtime.TransactionLocation(txID[:])

	err := vm.runtime.ExecuteTransaction(i.tx.Script, i.tx.Arguments, env, location)

	return createInvocationResult(txID, nil, env.getEvents(), env.getLogs(), err)
}

func checkAndIncrementSequenceNumber(ledger Ledger, proposalKey flow.ProposalKey) error {
	accountKey, err := getAccountPublicKey(ledger, proposalKey.Address, proposalKey.KeyID)
	if err != nil {
		if errors.Is(err, ErrAccountPublicKeyNotFound) {
			return &InvalidProposalKeyPublicKeyDoesNotExistError{
				Address:  proposalKey.Address,
				KeyIndex: proposalKey.KeyID,
			}
		}

		return err
	}

	if accountKey.Revoked {
		return &InvalidProposalKeyPublicKeyRevokedError{
			Address:  proposalKey.Address,
			KeyIndex: proposalKey.KeyID,
		}
	}

	valid := accountKey.SeqNumber == proposalKey.SequenceNumber

	if !valid {
		return &InvalidProposalKeySequenceNumberError{
			Address:           proposalKey.Address,
			KeyIndex:          proposalKey.KeyID,
			CurrentSeqNumber:  accountKey.SeqNumber,
			ProvidedSeqNumber: proposalKey.SequenceNumber,
		}
	}

	accountKey.SeqNumber++

	_, err = setAccountPublicKey(ledger, proposalKey.Address, proposalKey.KeyID, accountKey)
	if err != nil {
		return err
	}

	return nil
}

// verifySignatures verifies that a transaction contains the necessary signatures.
//
// An error is returned if any of the expected signatures are invalid or missing.
func verifySignatures(ledger Ledger, tx *flow.TransactionBody) (err error) {
	if tx.Payer == flow.EmptyAddress {
		return &MissingPayerError{}
	}

	var payloadWeights map[flow.Address]int
	var proposalKeyVerifiedInPayload bool

	payloadWeights, proposalKeyVerifiedInPayload, err = aggregateAccountSignatures(
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

	envelopeWeights, proposalKeyVerifiedInEnvelope, err = aggregateAccountSignatures(
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

		if !hasSufficientKeyWeight(payloadWeights, addr) {
			return &MissingSignatureError{addr}
		}
	}

	if !hasSufficientKeyWeight(envelopeWeights, tx.Payer) {
		return &MissingSignatureError{tx.Payer}
	}

	return nil
}

func aggregateAccountSignatures(
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
		accountKey, err := verifyAccountSignature(ledger, txSig, message)
		if err != nil {
			return nil, false, err
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
) (*flow.AccountPublicKey, error) {
	accountKey, err := getAccountPublicKey(ledger, txSig.Address, txSig.KeyID)
	if err != nil {
		if errors.Is(err, ErrAccountPublicKeyNotFound) {
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

	hasher := newHasher(accountKey.HashAlgo)
	if hasher == nil {
		return nil, &InvalidHashAlgorithmError{
			Address:  txSig.Address,
			KeyID:    txSig.KeyID,
			HashAlgo: accountKey.HashAlgo,
		}
	}

	valid, err := accountKey.PublicKey.Verify(txSig.Signature, message, hasher)
	if err != nil {
		return nil, err
	}

	if !valid {
		return nil, &InvalidSignatureVerificationError{Address: txSig.Address, KeyIndex: txSig.KeyID}

	}

	return &accountKey, nil
}

func sigIsForProposalKey(txSig flow.TransactionSignature, proposalKey flow.ProposalKey) bool {
	return txSig.Address == proposalKey.Address && txSig.KeyID == proposalKey.KeyID
}

func hasSufficientKeyWeight(weights map[flow.Address]int, address flow.Address) bool {
	return weights[address] >= AccountKeyWeightThreshold
}

func newHasher(hashAlgo hash.HashingAlgorithm) hash.Hasher {
	switch hashAlgo {
	case hash.SHA2_256:
		return hash.NewSHA2_256()
	case hash.SHA3_256:
		return hash.NewSHA3_256()
	}

	return nil
}
