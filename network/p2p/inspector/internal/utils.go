package internal

import (
	"crypto/rand"
	"encoding/base64"
)

// Nonce returns random string that is used to store unique items in herocache.
func Nonce() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}
