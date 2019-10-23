package utils

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/dapperlabs/flow-go/crypto"
)

func Exit(code int, msg string) {
	fmt.Println(msg)
	os.Exit(code)
}

func Exitf(code int, msg string, args ...interface{}) {
	fmt.Printf(msg+"\n", args...)
	os.Exit(code)
}

func DecodePrivateKey(derHex string) (crypto.PrivateKey, error) {
	prKeyDer, err := hex.DecodeString(derHex)
	if err != nil {
		return nil, err
	}

	return crypto.DecodePrivateKey(crypto.ECDSA_P256, prKeyDer)
}

func EncodePrivateKey(sk crypto.PrivateKey) (string, error) {
	skBytes, err := sk.Encode()
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(skBytes), nil
}

func EncodePublicKey(pubKey crypto.PublicKey) ([]byte, error) {
	return pubKey.Encode()
}
