//go:build !relic
// +build !relic

package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test for all public APIs requiring relic build tag.
// These functions should panic if build without the relic tag.
func TestNoRelicPanic(t *testing.T) {
	assert.PanicsWithValue(t, relic_panic, func() { NewBLSKMAC("") })
	assert.PanicsWithValue(t, relic_panic, func() { BLSInvalidSignature() })
	assert.PanicsWithValue(t, relic_panic, func() { BLSGeneratePOP(nil) })
	assert.PanicsWithValue(t, relic_panic, func() { BLSVerifyPOP(nil, nil) })
	assert.PanicsWithValue(t, relic_panic, func() { AggregateBLSSignatures(nil) })
	assert.PanicsWithValue(t, relic_panic, func() { AggregateBLSPrivateKeys(nil) })
	assert.PanicsWithValue(t, relic_panic, func() { AggregateBLSPublicKeys(nil) })
	assert.PanicsWithValue(t, relic_panic, func() { NeutralBLSPublicKey() })
	assert.PanicsWithValue(t, relic_panic, func() { RemoveBLSPublicKeys(nil, nil) })
	assert.PanicsWithValue(t, relic_panic, func() { VerifyBLSSignatureOneMessage(nil, nil, nil, nil) })
	assert.PanicsWithValue(t, relic_panic, func() { VerifyBLSSignatureManyMessages(nil, nil, nil, nil) })
	assert.PanicsWithValue(t, relic_panic, func() { BatchVerifyBLSSignaturesOneMessage(nil, nil, nil, nil) })
	assert.PanicsWithValue(t, relic_panic, func() { NewBLSThresholdSignatureParticipant(nil, nil, 0, 0, nil, nil, "") })
	assert.PanicsWithValue(t, relic_panic, func() { NewBLSThresholdSignatureInspector(nil, nil, 0, nil, "") })
	assert.PanicsWithValue(t, relic_panic, func() { BLSReconstructThresholdSignature(0, 0, nil, nil) })
	assert.PanicsWithValue(t, relic_panic, func() { EnoughShares(0, 0) })
	assert.PanicsWithValue(t, relic_panic, func() { BLSThresholdKeyGen(0, 0, nil) })
	assert.PanicsWithValue(t, relic_panic, func() { NewFeldmanVSS(0, 0, 0, nil, 0) })
	assert.PanicsWithValue(t, relic_panic, func() { NewFeldmanVSSQual(0, 0, 0, nil, 0) })
	assert.PanicsWithValue(t, relic_panic, func() { NewJointFeldman(0, 0, 0, nil) })
}
