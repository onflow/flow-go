package request

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
