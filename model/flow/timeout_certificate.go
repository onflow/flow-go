package flow

// TimeoutCertificate represents a timeout certificate for some view.
// A timeout certificate is a collection of special votes(called timeouts) which describe intent of committee to change active view.
// Valid timeout certificates contain signatures from a super-majority of consensus committee members and
// max(TOHighQCViews) == TOHighestQC.View
type TimeoutCertificate struct {
	View uint64
	// TOHighQCViews represents HighestQC.View for each replica, meaning that every replica that
	// contributes to TimeoutCertificate it shares it's local highest QC view
	TOHighQCViews []uint64
	// TOHighestQC is the highest QC  over all replicas that contributed to this certificate.
	TOHighestQC *QuorumCertificate
	// SignerIDs holds the IDs of HotStuff participants that voted for the block.
	SignerIDs []Identifier
	// SigData is an aggregated signature over multiple messages, each signed by different replica,
	// each message consists of (View, HighestQCView).
	SigData []byte
}
