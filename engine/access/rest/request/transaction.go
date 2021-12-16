package request

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"io"
)

const maxAuthorizers = 100
const maxAllowedScriptArguments = 100

type transactionBody struct {
	Script             string                     `json:"script"`
	Arguments          []string                   `json:"arguments"`
	ReferenceBlockId   string                     `json:"reference_block_id"`
	GasLimit           string                     `json:"gas_limit"`
	Payer              string                     `json:"payer"`
	ProposalKey        *proposalKeyBody           `json:"proposal_key"`
	Authorizers        []string                   `json:"authorizers"`
	PayloadSignatures  []transactionSignatureBody `json:"payload_signatures"`
	EnvelopeSignatures []transactionSignatureBody `json:"envelope_signatures"`
}

type Transaction flow.TransactionBody

func (t *Transaction) Parse(raw io.Reader) error {
	var tx transactionBody
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
	if len(tx.Authorizers) == 0 {
		return fmt.Errorf("authorizers not provided")
	}
	if len(tx.Authorizers) > maxAuthorizers {
		return fmt.Errorf("too many authorizers. Maximum authorizers allowed: %d", maxAuthorizers)
	}
	if len(tx.Arguments) > maxAllowedScriptArguments {
		return fmt.Errorf("too many arguments. Maximum arguments allowed: %d", maxAllowedScriptArguments)
	}
	if len(tx.EnvelopeSignatures) == 0 {
		return fmt.Errorf("envelope signatures not provided")
	}
	if tx.ReferenceBlockId == "" {
		return fmt.Errorf("reference block not provided")
	}
	if len(tx.EnvelopeSignatures) == 0 {
		return fmt.Errorf("envelope signatures not provided")
	}
	if tx.ReferenceBlockId == "" {
		return fmt.Errorf("reference block not provided")
	}

	var args Arguments
	err = args.Parse(tx.Arguments)
	if err != nil {
		return err
	}

	var payer Address
	err = payer.Parse(tx.Payer)
	if err != nil {
		return err
	}

	auths := make([]flow.Address, len(tx.Authorizers))
	for i, auth := range tx.Authorizers {
		var a Address
		err := a.Parse(auth)
		if err != nil {
			return err
		}

		auths[i] = a.Flow()
	}

	var proposal ProposalKey
	err = proposal.Parse(*tx.ProposalKey)
	if err != nil {
		return err
	}

	var payloadSigs TransactionSignatures
	err = payloadSigs.Parse(tx.PayloadSignatures)
	if err != nil {
		return err
	}

	var envelopeSigs TransactionSignatures
	err = envelopeSigs.Parse(tx.EnvelopeSignatures)
	if err != nil {
		return err
	}

	// script comes in as a base64 encoded string, decode base64 back to a string here
	script, err := fromBase64(tx.Script)
	if err != nil {
		return fmt.Errorf("invalid transaction script encoding")
	}

	var blockID ID
	err = blockID.Parse(tx.ReferenceBlockId)
	if err != nil {
		return err
	}

	gasLimit, err := toUint64(tx.GasLimit)
	if err != nil {
		return fmt.Errorf("invalid value for gas limit")
	}

	*t = Transaction(flow.TransactionBody{
		ReferenceBlockID:   blockID.Flow(),
		Script:             script,
		Arguments:          args.Flow(),
		GasLimit:           gasLimit,
		ProposalKey:        proposal.Flow(),
		Payer:              payer.Flow(),
		Authorizers:        auths,
		PayloadSignatures:  payloadSigs.Flow(),
		EnvelopeSignatures: envelopeSigs.Flow(),
	})

	return nil
}

func (t Transaction) Flow() flow.TransactionBody {
	return flow.TransactionBody(t)
}
