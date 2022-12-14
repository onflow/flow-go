package main

import (
	"crypto/rand"
	b64 "encoding/base64"
	"fmt"

	"github.com/onflow/flow-go-sdk/crypto"
)

func main() {
	// Generate key
	seed := make([]byte, crypto.MinSeedLength)
	_, err := rand.Read(seed)
	if err != nil {
		panic(err)
	}

	sk, err := crypto.GeneratePrivateKey(crypto.ECDSA_P256, seed)
	if err != nil {
		panic(err)
	}
	fmt.Println("RAW Private KEY")
	fmt.Println(sk.String())
	fmt.Println("Encoded Private KEY")
	fmt.Println(b64.StdEncoding.EncodeToString(sk.Encode()))

	fmt.Println("RAW PUBLIC KEY")
	fmt.Println(sk.PublicKey())
	a := sk.PublicKey()
	sEnc := b64.StdEncoding.EncodeToString(a.Encode())
	fmt.Println("Encoded Public Key")
	fmt.Println(sEnc)
}
