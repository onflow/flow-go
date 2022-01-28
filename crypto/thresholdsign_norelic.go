//go:build !relic
// +build !relic

// Stubs of thresholdsign.go for non relic build of flow-go.

package crypto

type ThresholdSignatureInspector interface {
	VerifyShare(orig int, share Signature) (bool, error)
	VerifyThresholdSignature(thresholdSignature Signature) (bool, error)
	EnoughShares() bool
	TrustedAdd(orig int, share Signature) (bool, error)
	VerifyAndAdd(orig int, share Signature) (bool, bool, error)
	HasShare(orig int) (bool, error)
	ThresholdSignature() (Signature, error)
}

type ThresholdSignatureParticipant interface {
	ThresholdSignatureInspector
	SignShare() (Signature, error)
}

func ReconstructThresholdSignature(_ int, _ int,
	_ []Signature, _ []int) (Signature, error) {
	panic("ReconstructThresholdSignature not supported when flow-go is built without relic")
}

func ThresholdSignKeyGen(_ int, _ int, _ []byte) ([]PrivateKey,
	[]PublicKey, PublicKey, error) {
	panic("ThresholdSignKeyGen not supported when flow-go is built without relic")
}
