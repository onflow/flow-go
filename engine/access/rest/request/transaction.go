package request

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"io"
)

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

type proposalKeyBody struct {
	Address        string `json:"address"`
	KeyIndex       string `json:"key_index"`
	SequenceNumber string `json:"sequence_number"`
}

type transactionSignatureBody struct {
	Address     string `json:"address"`
	SignerIndex string `json:"signer_index"`
	KeyIndex    string `json:"key_index"`
	Signature   string `json:"signature"`
}

type Transaction flow.TransactionBody

func (t *Transaction) Parse(raw io.Reader) error {
	var tx transactionBody
	err := parseBody(raw, &tx)
	if err != nil {
		return err
	}

}

func toTransaction(tx transactionBody) (flow.TransactionBody, error) {
	//argLen := len(tx.Arguments)
	//if argLen > maxAllowedScriptArgumentsCnt {
	//	return flow.TransactionBody{}, fmt.Errorf("too many arguments. Maximum arguments allowed: %d", maxAllowedScriptArgumentsCnt)
	//}

	if tx.ProposalKey == nil {
		return flow.TransactionBody{}, fmt.Errorf("proposal key not provided")
	}
	if tx.Script == "" {
		return flow.TransactionBody{}, fmt.Errorf("script not provided")
	}
	if tx.Payer == "" {
		return flow.TransactionBody{}, fmt.Errorf("payer not provided")
	}
	if len(tx.Authorizers) == 0 {
		return flow.TransactionBody{}, fmt.Errorf("authorizers not provided")
	}
	//if len(tx.Authorizers) > maxAuthorizersCnt {
	//	return flow.TransactionBody{}, fmt.Errorf("too many authorizers. Maximum authorizers allowed: %d", maxAuthorizersCnt)
	//}
	if len(tx.EnvelopeSignatures) == 0 {
		return flow.TransactionBody{}, fmt.Errorf("envelope signatures not provided")
	}
	if tx.ReferenceBlockId == "" {
		return flow.TransactionBody{}, fmt.Errorf("reference block not provided")
	}

	// script arguments come in as a base64 encoded strings, decode base64 back to a string here
	var args [][]byte
	for i, arg := range tx.Arguments {
		decodedArg, err := fromBase64(arg)
		if err != nil {
			return flow.TransactionBody{}, fmt.Errorf("invalid argument encoding with index: %d", i)
		}
		args = append(args, decodedArg)
	}

	proposal, err := toProposalKey(tx.ProposalKey)
	if err != nil {
		return flow.TransactionBody{}, err
	}

	payer, err := toAddress(tx.Payer)
	if err != nil {
		return flow.TransactionBody{}, err
	}

	auths := make([]flow.Address, len(tx.Authorizers))
	for i, auth := range tx.Authorizers {
		a, err := toAddress(auth)
		if err != nil {
			return flow.TransactionBody{}, err
		}

		auths[i] = a
	}

	payloadSigs, err := toTransactionSignatures(tx.PayloadSignatures)
	if err != nil {
		return flow.TransactionBody{}, err
	}

	envelopeSigs, err := toTransactionSignatures(tx.EnvelopeSignatures)
	if err != nil {
		return flow.TransactionBody{}, err
	}

	// script comes in as a base64 encoded string, decode base64 back to a string here
	script, err := fromBase64(tx.Script)
	if err != nil {
		return flow.TransactionBody{}, fmt.Errorf("invalid transaction script encoding")
	}

	blockID, err := toID(tx.ReferenceBlockId)
	if err != nil {
		return flow.TransactionBody{}, err
	}

	gasLimit, err := toUint64(tx.GasLimit)
	if err != nil {
		return flow.TransactionBody{}, fmt.Errorf("invalid value for gas limit")
	}

	return flow.TransactionBody{
		ReferenceBlockID:   blockID,
		Script:             script,
		Arguments:          args,
		GasLimit:           gasLimit,
		ProposalKey:        proposal,
		Payer:              payer,
		Authorizers:        auths,
		PayloadSignatures:  payloadSigs,
		EnvelopeSignatures: envelopeSigs,
	}, nil
}
