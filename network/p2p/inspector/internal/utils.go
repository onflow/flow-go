package internal

import (
	"crypto/rand"
)

// Nonce returns random string that is used to store unique items in herocache.
func Nonce() ([]byte, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}
