package coa

import (
	_ "embed"
	"encoding/hex"
)

var ContractDeploymentRequiredGas = uint64(723_000)

//go:embed coa_bytes.hex
var contractBytesInHex string

// ContractBytes is the compiled version of the coa smart contract.
var ContractBytes, _ = hex.DecodeString(contractBytesInHex)

// ContractABIJSON is the json string of ABI of the coa smart contract.
//
//go:embed coa_abi.json
var ContractABIJSON string
