package flow

import "github.com/onflow/flow-go/crypto"

// TimeoutCertificate proves that a super-majority of consensus participants want to abandon the specified View.
// At its core, a timeout certificate is an aggregation of TimeoutObjects, which individual nodes send to signal
// their intent to leave the active view.
type TimeoutCertificate struct {
	View uint64
	// NewestQCViews lists for each signer (in the same order) the view of the newest QC they supplied
	// as part of their TimeoutObject message (specifically TimeoutObject.NewestQC.View).
	NewestQCViews []uint64
	// NewestQC is the newest QC from all TimeoutObject that were aggregated for this certificate.
	NewestQC *QuorumCertificate
	// SignerIDs holds the IDs of all HotStuff participants whose TimeoutObject was included in this certificate
	SignerIDs []Identifier
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
		NewestQCViews []uint64
		NewestQC      QuorumCertificate
		SignerIDs     []Identifier
		SigData       crypto.Signature
	}{
		View:          t.View,
		NewestQCViews: t.NewestQCViews,
		NewestQC:      *t.NewestQC,
		SignerIDs:     t.SignerIDs,
		SigData:       t.SigData,
	}
}
