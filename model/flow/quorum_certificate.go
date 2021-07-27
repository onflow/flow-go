package flow

// QuorumCertificate represents a quorum certificate for a block proposal as defined in the HotStuff algorithm.
// A quorum certificate is a collection of votes for a particular block proposal. Valid quorum certificates contain
// signatures from a super-majority of consensus committee members.
type QuorumCertificate struct {
	View    uint64
	BlockID Identifier
	// For consensus cluster, the signerIDs includes both threshold sig signers and staking sig signers.
	// In order to distinguish them, the SigData has to be deseralized and a bit vector, namely the
	// SigType field, can be used to distinguish whether each signer signed threshold sig or staking sig.
	// For collection cluster, the SignerIDs includes all the staking sig signers.
	SignerIDs []Identifier
	// For consensus cluster, the SigData is a serialization of the following fields
	// - SigType []byte, bit-vector indicating the type of sig produced by the signer.
	// - AggregatedStakingSig crypto.Signature,
	// - AggregatedRandomBeaconSig crypto.Signature
	// - RandomBeacon crypto.Signature
	SigData []byte
}
