package utils

import (
	"encoding/hex"

	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

func DecodePrivateKey(derHex string) (crypto.PrKey, error) {
	salg, err := crypto.NewSignatureAlgo(crypto.ECDSA_P256)
	if err != nil {
		return nil, err
	}

	prKeyDer, err := hex.DecodeString(derHex)
	if err != nil {
		return nil, err
	}

	return salg.DecodePrKey(prKeyDer)
}
