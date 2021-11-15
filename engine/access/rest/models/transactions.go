package models

import (
	"encoding/json"
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"io"
	"regexp"
)

type transactionSignatureRequest struct {
	Address     string `json:"address"`
	SignerIndex int32  `json:"signer_index"`
	KeyIndex    int32  `json:"key_index"`
	Signature   string `json:"signature"`
}

type addressRequest string

func (a *addressRequest) ToFlow() (flow.Address, error) {
	addr := string(*a)
	valid, _ := regexp.MatchString(`^[0-9a-fA-F]{16}$`, addr)
	if !valid {
		return flow.Address{}, fmt.Errorf("invalid address")
	}

	return flow.HexToAddress(addr), nil
}

type proposalKeyRequest struct {
	Address        addressRequest `json:"address"`
	KeyIndex       int32          `json:"key_index"`
	SequenceNumber int32          `json:"sequence_number"`
}

// ToFlow converts proposal key request to flow proposal key.
func (p *proposalKeyRequest) ToFlow() (flow.ProposalKey, error) {
	address, err := p.Address.ToFlow()
	if err != nil {
		return flow.ProposalKey{}, err
	}

	return flow.ProposalKey{
		Address:        address,
		KeyIndex:       uint64(p.KeyIndex),
		SequenceNumber: uint64(p.SequenceNumber),
	}, nil
}

type TransactionRequest struct {
	Id                 string                        `json:"id"`
	Script             string                        `json:"script"`
	Arguments          []string                      `json:"arguments"`
	ReferenceBlockId   string                        `json:"reference_block_id"`
	GasLimit           int32                         `json:"gas_limit"`
	Payer              addressRequest                `json:"payer"`
	ProposalKey        *proposalKeyRequest           `json:"proposal_key,omitempty"`
	Authorizers        []string                      `json:"authorizers,omitempty"`
	PayloadSignatures  []transactionSignatureRequest `json:"payload_signatures,omitempty"`
	EnvelopeSignatures []transactionSignatureRequest `json:"envelope_signatures,omitempty"`
}

// ToFlow converts transaction request to flow transaction.
func (t *TransactionRequest) ToFlow() (*flow.TransactionBody, error) {
	var args [][]byte
	for _, arg := range t.Arguments {
		// todo validate
		args = append(args, []byte(arg))
	}

	proposalKey, err := t.ProposalKey.ToFlow()
	if err != nil {
		return nil, err
	}

	payer, err := t.Payer.ToFlow()
	if err != nil {
		return nil, err
	}

	return &flow.TransactionBody{
		ReferenceBlockID:   flow.Identifier{},
		Script:             []byte(t.Script),
		Arguments:          args,
		GasLimit:           uint64(t.GasLimit),
		ProposalKey:        proposalKey,
		Payer:              payer,
		Authorizers:        nil,
		PayloadSignatures:  nil,
		EnvelopeSignatures: nil,
	}, nil
}

// TransactionFromRequest creates a flow transaction from request payload and validates data.
func TransactionFromRequest(body io.ReadCloser) (*flow.TransactionBody, error) {
	var tr TransactionRequest
	err := decodeJSON(body, tr)
	if err != nil {
		return nil, err
	}

	return tr.ToFlow()
}

type transactionResponse struct {
	Id               string   `json:"id"`
	Script           string   `json:"script"`
	Arguments        []string `json:"arguments"`
	ReferenceBlockId string   `json:"reference_block_id"`
	GasLimit         int32    `json:"gas_limit"`
	Payer            string   `json:"payer"`
}

// TransactionToJSON converts flow transaction to JSON data.
func TransactionToJSON(tx *flow.TransactionBody, expandable []string) ([]byte, error) {
	// handle expandable
	return json.Marshal(
		transactionResponse{
			Id:               tx.ID().String(),
			Script:           string(tx.Script),
			Arguments:        nil,
			ReferenceBlockId: "",
			GasLimit:         0,
			Payer:            "",
		},
	)
}
