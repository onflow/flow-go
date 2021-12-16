package request

type Transactions struct {
	Script             string                 `json:"script"`
	Arguments          []string               `json:"arguments"`
	ReferenceBlockId   string                 `json:"reference_block_id"`
	GasLimit           string                 `json:"gas_limit"`
	Payer              string                 `json:"payer"`
	ProposalKey        *ProposalKey           `json:"proposal_key"`
	Authorizers        []string               `json:"authorizers"`
	PayloadSignatures  []TransactionSignature `json:"payload_signatures"`
	EnvelopeSignatures []TransactionSignature `json:"envelope_signatures"`
}
