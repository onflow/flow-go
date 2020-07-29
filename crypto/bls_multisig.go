// +build relic

package crypto

// BLS multi signature using BLS12-381 curve
// ([zcash]https://github.com/zkcrypto/pairing/blob/master/src/bls12_381/README.md#bls12-381)
// Pairing, ellipic curve and modular arithmetic is using Relic library.
// This implementation does not include any security against side-channel attacks.

// existing features:
//  - the same BLS set-up in bls.go
//  - Use the proof of possession (PoP) to prevent the Rogue public-key attack.

// #cgo CFLAGS: -g -Wall -std=c99 -I./ -I./relic/build/include
// #cgo LDFLAGS: -Lrelic/build/lib -l relic_s
// #include "bls_include.h"
import "C"
