//go:build !relic
// +build !relic

// Stubs of bls_thresholdsign_relic.go for non relic build of flow-go.

package crypto

func NewBLSThresholdSignatureParticipant(
	_ PublicKey,
	_ []PublicKey,
	_ int,
	_ int,
	_ PrivateKey,
	_ []byte,
	_ string,
) (ThresholdSignatureInspector, error) {
	panic("NewBLSThresholdSignatureParticipant not supported when flow-go is built without relic")
}

func NewBLSThresholdSignatureInspector(
	_ PublicKey,
	_ []PublicKey,
	_ int,
	_ []byte,
	_ string,
) (ThresholdSignatureInspector, error) {
	panic("NewBLSThresholdSignatureInspector not supported when flow-go is built without relic")
}

func EnoughShares(_ int, _ int) (bool, error) {
	panic("EnoughShares not supported when flow-go is built without relic")
}

func BLSThresholdKeyGen(_ int, _ int, _ []byte) ([]PrivateKey,
	[]PublicKey, PublicKey, error) {
	panic("BLSThresholdKeyGen not supported when flow-go is built without relic")
}
