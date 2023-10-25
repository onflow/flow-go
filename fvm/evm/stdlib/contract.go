package stdlib

import _ "embed"

//go:embed contract.cdc
var contractContent string

func ContractName() string {
	return "EVM"
}

func ContractContent() []byte {
	return []byte(contractContent)
}
