package client

import "math/rand"

func RandomNonce() uint64 {
	return rand.Uint64()
}
