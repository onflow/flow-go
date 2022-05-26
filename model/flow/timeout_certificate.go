package flow

import "github.com/onflow/flow-go/crypto"

// TimeoutCertificate proves that a super-majority of consensus participants want to abandon the specified View.
// At its core, a timeout certificate is an aggregation of TimeoutObjects, which individual nodes send to signal
// their intent to leave the active view.
type TimeoutCertificate struct {
	View uint64
	// TOHighQCViews lists for each signer (in the same order) the view of the highest QC they supplied
	// as part of their TimeoutObject message (specifically TimeoutObject.HighestQC.View).
	TOHighQCViews []uint64
	// TOHighestQC is the newest QC from all TimeoutObject that were aggregated for this certificate.
	TOHighestQC *QuorumCertificate
	// SignerIndices encodes the HotStuff participants whose TimeoutObjects are included in this TC.
	// For `n` authorized consensus nodes, `SignerIndices` is an n-bit vector (padded with tailing
	// zeros to reach full bytes). We list the nodes in their canonical order, as defined by the protocol.
	SignerIndices []byte
	// SigData is an aggregated signature from multiple TimeoutObjects, each from a different replica.
	// In their TimeoutObjects, replicas sign the pair (View, HighestQCView) with their staking keys.
	SigData crypto.Signature
}

func (t *TimeoutCertificate) Body() interface{} {
	if t == nil {
		return struct{}{}
	}

	return struct {
		View          uint64
		HighQCViews   []uint64
		HighestQC     QuorumCertificate
		SignerIndices []byte
		SigData       crypto.Signature
	}{
		View:          t.View,
		HighQCViews:   t.TOHighQCViews,
		HighestQC:     *t.TOHighestQC,
		SignerIndices: t.SignerIndices,
		SigData:       t.SigData,
	}
}
