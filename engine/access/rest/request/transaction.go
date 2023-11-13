package request

import (
	"fmt"
	"io"

	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
)

const maxAuthorizers = 100
const maxAllowedScriptArguments = 100

type Transaction flow.TransactionBody

func (t *Transaction) Parse(raw io.Reader, chain flow.Chain) error {
	var tx models.TransactionsBody
	err := parseBody(raw, &tx)
	if err != nil {
		return err
	}

	if tx.ProposalKey == nil {
		return fmt.Errorf("proposal key not provided")
	}
	if tx.Script == "" {
		return fmt.Errorf("script not provided")
	}
	if tx.Payer == "" {
		return fmt.Errorf("payer not provided")
	}
	if len(tx.Authorizers) > maxAuthorizers {
		return fmt.Errorf("too many authorizers. Maximum authorizers allowed: %d", maxAuthorizers)
	}
	if len(tx.Arguments) > maxAllowedScriptArguments {
		return fmt.Errorf("too many arguments. Maximum arguments allowed: %d", maxAllowedScriptArguments)
	}
	if tx.ReferenceBlockId == "" {
		return fmt.Errorf("reference block not provided")
	}
	if len(tx.EnvelopeSignatures) == 0 {
		return fmt.Errorf("envelope signatures not provided")
	}

	var args Arguments
	err = args.Parse(tx.Arguments)
	if err != nil {
		return err
	}

	payer, err := ParseAddress(tx.Payer, chain)
	if err != nil {
		return fmt.Errorf("invalid payer: %w", err)
	}

	auths := make([]flow.Address, len(tx.Authorizers))
	for i, auth := range tx.Authorizers {
		a, err := ParseAddress(auth, chain)
		if err != nil {
			return err
		}

		auths[i] = a
	}

	var proposal ProposalKey
	err = proposal.Parse(*tx.ProposalKey, chain)
	if err != nil {
		return err
	}

	var payloadSigs TransactionSignatures
	err = payloadSigs.Parse(tx.PayloadSignatures, chain)
	if err != nil {
		return err
	}

	var envelopeSigs TransactionSignatures
	err = envelopeSigs.Parse(tx.EnvelopeSignatures, chain)
	if err != nil {
		return err
	}

	// script comes in as a base64 encoded string, decode base64 back to a string here
	script, err := util.FromBase64(tx.Script)
	if err != nil {
		return fmt.Errorf("invalid transaction script encoding")
	}

	var blockID ID
	err = blockID.Parse(tx.ReferenceBlockId)
	if err != nil {
		return fmt.Errorf("invalid reference block ID: %w", err)
	}

	gasLimit, err := util.ToUint64(tx.GasLimit)
	if err != nil {
		return fmt.Errorf("invalid gas limit: %w", err)
	}

	flowTransaction := flow.TransactionBody{
		ReferenceBlockID:   blockID.Flow(),
		Script:             script,
		Arguments:          args.Flow(),
		GasLimit:           gasLimit,
		ProposalKey:        proposal.Flow(),
		Payer:              payer,
		Authorizers:        auths,
		PayloadSignatures:  payloadSigs.Flow(),
		EnvelopeSignatures: envelopeSigs.Flow(),
	}

	// we use the gRPC method of converting the incoming transaction to a Flow transaction since
	// it sets the signer_index appropriately.
	entityTransaction := convert.TransactionToMessage(flowTransaction)
	flowTx, err := convert.MessageToTransaction(entityTransaction, chain)
	if err != nil {
		return err
	}

	*t = Transaction(flowTx)

	return nil
}

func (t Transaction) Flow() flow.TransactionBody {
	return flow.TransactionBody(t)
}
