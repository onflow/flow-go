package flow

// QuorumCertificate represents a quorum certificate for a block proposal as defined in the HotStuff algorithm.
// A quorum certificate is a collection of votes for a particular block proposal. Valid quorum certificates contain
// signatures from a super-majority of consensus committee members.
type QuorumCertificate struct {
	View    uint64
	BlockID Identifier

	// SignerIDs holds the IDs of HotStuff participants that voted for the block.
	// Note that for the main consensus committee, members can provide a staking or a threshold signature
	// to indicate their HotStuff vote. In addition to contributing to consensus progress, committee members
	// contribute to running the Random Beacon if they express their vote through a threshold signature.
	// In order to distinguish the signature types, the SigData has to be deserialized. Specifically,
	// the field `SigData.SigType` (bit vector) indicates for each signer which sig type they provided.
	// For collection cluster, the SignerIDs includes all the staking sig signers.
	SignerIDs []Identifier

	// For consensus cluster, the SigData is a serialization of the following fields
	// - SigType []byte, bit-vector indicating the type of sig produced by the signer.
	// - AggregatedStakingSig crypto.Signature,
	// - AggregatedRandomBeaconSig crypto.Signature
	// - ReconstrcutedRandomBeaconSig crypto.Signature
	// For collector cluster HotStuff, SigData is simply the aggregated staking signatures
	// from all signers.
	SigData []byte
}
