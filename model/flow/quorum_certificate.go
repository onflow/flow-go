package flow

// QuorumCertificate represents a quorum certificate for a block proposal as defined in the HotStuff algorithm.
// A quorum certificate is a collection of votes for a particular block proposal. Valid quorum certificates contain
// signatures from a super-majority of consensus committee members.
type QuorumCertificate struct {
	View      uint64
	BlockID   Identifier
	SignerIDs []Identifier
	SigData   []byte
}
