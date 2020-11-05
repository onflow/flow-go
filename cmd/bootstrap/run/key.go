package run

import (
	"fmt"

	"github.com/onflow/flow-go/crypto"
)

func ReadPrivateKeyFromPath(path string) (crypto.PrivateKey, error) {
	// validation
	return nil, fmt.Errorf("not implemented")
}

func WritePrivateKeyToPath(path string, key crypto.PrivateKey) error {
	return fmt.Errorf("not implemented")
}

func ReadPublicKeyFromPath(path string) (crypto.PublicKey, error) {
	// validation
	return nil, fmt.Errorf("not implemented")
}

func WritePublicKeyToPath(path string, key crypto.PublicKey) error {
	return fmt.Errorf("not implemented")
}
