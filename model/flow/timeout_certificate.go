package flow

import "github.com/onflow/crypto"

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
	// SignerIndices encodes the HotStuff participants whose TimeoutObjects are included in this TC.
	// For `n` authorized consensus nodes, `SignerIndices` is an n-bit vector (padded with tailing
	// zeros to reach full bytes). We list the nodes in their canonical order, as defined by the protocol.
	SignerIndices []byte
	// SigData is an aggregated signature from multiple TimeoutObjects, each from a different replica.
	// In their TimeoutObjects, replicas sign the pair (View, NewestQCView) with their staking keys.
	SigData crypto.Signature
}

// ID returns the TimeoutCertificate's identifier
func (t *TimeoutCertificate) ID() Identifier {
	if t == nil {
		return ZeroID
	}

	body := struct {
		View          uint64
		NewestQCViews []uint64
		NewestQCID    Identifier
		SignerIndices []byte
		SigData       crypto.Signature
	}{
		View:          t.View,
		NewestQCViews: t.NewestQCViews,
		NewestQCID:    t.NewestQC.ID(),
		SignerIndices: t.SignerIndices,
		SigData:       t.SigData,
	}
	return MakeID(body)
}
