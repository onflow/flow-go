package utils

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/dapperlabs/flow-go/pkg/crypto"
)

func Exit(msg string, code int) {
	fmt.Println(msg)
	os.Exit(1)
}

func DecodePrivateKey(derHex string) (crypto.PrivateKey, error) {
	prKeyDer, err := hex.DecodeString(derHex)
	if err != nil {
		return nil, err
	}

	return crypto.DecodePrivateKey(crypto.ECDSA_P256, prKeyDer)
}

func EncodePrKey(sk crypto.PrivateKey) (string, error) {
	skBytes, err := sk.Encode()
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(skBytes), nil
}

func EncodePublicKey(pubKey crypto.PublicKey) ([]byte, error) {
	return pubKey.Encode()
}
