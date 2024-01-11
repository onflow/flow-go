package main

import (
	"crypto/ecdsa"
	"strings"
)

type TransactionGenerationContext struct {
	AddressReplacements map[string]string

	ETHAddressNonce      uint64
	EthAddressPrivateKey *ecdsa.PrivateKey
}

func (ctx *TransactionGenerationContext) ReplaceAddresses(tx string) string {
	for key, value := range ctx.AddressReplacements {
		tx = strings.ReplaceAll(tx, key, value)
	}
	return tx
}

func (ctx *TransactionGenerationContext) GetETHAddressNonce() uint64 {
	nonce := ctx.ETHAddressNonce
	ctx.ETHAddressNonce++
	return nonce
}
