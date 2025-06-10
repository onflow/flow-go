package flow

import (
	"bytes"

	"github.com/onflow/crypto"
)

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

// Equals returns true if and only if receiver TimeoutCertificate is equal to the `other`. Nil values are supported.
func (t *TimeoutCertificate) Equals(other *TimeoutCertificate) bool {
	// Shortcut if `t` and `other` point to the same object; covers case where both are nil.
	if t == other {
		return true
	}
	if t == nil || other == nil { // only one is nil, the other not (otherwise we would have returned above)
		return false
	}
	// both are not nil, so we can compare the fields
	if len(t.NewestQCViews) != len(other.NewestQCViews) {
		return false
	}
	for idx, v := range t.NewestQCViews {
		if v != other.NewestQCViews[idx] {
			return false
		}
	}
	return (t.View == other.View) &&
		t.NewestQC.Equals(other.NewestQC) &&
		bytes.Equal(t.SignerIndices, other.SignerIndices) &&
		bytes.Equal(t.SigData, other.SigData)
}

// ID returns the TimeoutCertificate's identifier
func (t *TimeoutCertificate) ID() Identifier {
	if t == nil {
		return ZeroID
	}
	return MakeID(t)
}
