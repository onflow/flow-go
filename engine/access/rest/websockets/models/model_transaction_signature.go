package models

type TransactionSignature struct {
	Address     string `json:"address"`
	SignerIndex string `json:"signer_index"`
	KeyIndex    string `json:"key_index"`
	Signature   string `json:"signature"`
}
