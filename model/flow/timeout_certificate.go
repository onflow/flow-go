package flow

import (
	"bytes"
	"fmt"

	"github.com/onflow/crypto"
)

// TimeoutCertificate proves that a super-majority of consensus participants want to abandon the specified View.
// At its core, a timeout certificate is an aggregation of TimeoutObjects, which individual nodes send to signal
// their intent to leave the active view.
//
//structwrite:immutable - mutations allowed only within the constructor
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

// UntrustedTimeoutCertificate is an untrusted input-only representation of a TimeoutCertificate,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedTimeoutCertificate should be validated and converted into
// a trusted TimeoutCertificate using NewTimeoutCertificate constructor.
type UntrustedTimeoutCertificate TimeoutCertificate

// NewTimeoutCertificate creates a new instance of TimeoutCertificate.
// Construction TimeoutCertificate allowed only within the constructor.
//
// All errors indicate a valid TimeoutCertificate cannot be constructed from the input.
func NewTimeoutCertificate(untrusted UntrustedTimeoutCertificate) (*TimeoutCertificate, error) {
	if untrusted.NewestQC == nil {
		return nil, fmt.Errorf("newest QC must not be nil")
	}
	if len(untrusted.SignerIndices) == 0 {
		return nil, fmt.Errorf("signer indices must not be empty")
	}
	if len(untrusted.SigData) == 0 {
		return nil, fmt.Errorf("signature must not be empty")
	}

	// The TC's view cannot be smaller than the view of the QC it contains.
	// Note: we specifically allow for the TC to have the same view as the highest QC.
	// This is useful as a fallback, because it allows replicas other than the designated
	// leader to also collect votes and generate a QC.
	if untrusted.View < untrusted.NewestQC.View {
		return nil, fmt.Errorf("TC's QC view (%d) cannot be newer than the TC's view (%d)", untrusted.NewestQC.View, untrusted.View)
	}

	// verifying that tc.NewestQC is the QC with the highest view.
	// Note: A byzantine TC could include `nil` for tc.NewestQCViews
	if len(untrusted.NewestQCViews) == 0 {
		return nil, fmt.Errorf("newest QC views must not be empty")
	}

	newestQCView := untrusted.NewestQCViews[0]
	for _, view := range untrusted.NewestQCViews {
		if newestQCView < view {
			newestQCView = view
		}
	}
	if newestQCView > untrusted.NewestQC.View {
		return nil, fmt.Errorf("included QC (view=%d) should be equal or higher to highest contributed view: %d", untrusted.NewestQC.View, newestQCView)
	}

	return &TimeoutCertificate{
		View:          untrusted.View,
		NewestQCViews: untrusted.NewestQCViews,
		NewestQC:      untrusted.NewestQC,
		SignerIndices: untrusted.SignerIndices,
		SigData:       untrusted.SigData,
	}, nil
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
