package flow

import (
	"bytes"
	"fmt"
)

// QuorumCertificate represents a quorum certificate for a block proposal as defined in the HotStuff algorithm.
// A quorum certificate is a collection of votes for a particular block proposal. Valid quorum certificates contain
// signatures from a super-majority of consensus committee members.
//
//structwrite:immutable - mutations allowed only within the constructor
type QuorumCertificate struct {
	View    uint64
	BlockID Identifier

	// SignerIndices encodes the HotStuff participants whose vote is included in this QC.
	// For `n` authorized consensus nodes, `SignerIndices` is an n-bit vector (padded with tailing
	// zeros to reach full bytes). We list the nodes in their canonical order, as defined by the protocol.
	SignerIndices []byte

	// For consensus cluster, the SigData is a serialization of the following fields
	// - SigType []byte, bit-vector indicating the type of sig produced by the signer.
	// - AggregatedStakingSig []byte
	// - AggregatedRandomBeaconSig []byte
	// - ReconstructedRandomBeaconSig crypto.Signature
	// For collector cluster HotStuff, SigData is simply the aggregated staking signatures
	// from all signers.
	SigData []byte
}

// UntrustedQuorumCertificate is an untrusted input-only representation of a QuorumCertificate,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedQuorumCertificate should be validated and converted into
// a trusted QuorumCertificate using NewQuorumCertificate constructor.
type UntrustedQuorumCertificate QuorumCertificate

// NewQuorumCertificate creates a new instance of QuorumCertificate.
// Construction of QuorumCertificate is allowed only within the constructor
//
// All errors indicate a valid QuorumCertificate cannot be constructed from the input.
func NewQuorumCertificate(untrusted UntrustedQuorumCertificate) (*QuorumCertificate, error) {
	if untrusted.BlockID == ZeroID {
		return nil, fmt.Errorf("BlockID must not be empty")
	}

	if len(untrusted.SignerIndices) == 0 {
		return nil, fmt.Errorf("SignerIndices must not be empty")
	}

	if len(untrusted.SigData) == 0 {
		return nil, fmt.Errorf("SigData must not be empty")
	}

	return &QuorumCertificate{
		View:          untrusted.View,
		BlockID:       untrusted.BlockID,
		SignerIndices: untrusted.SignerIndices,
		SigData:       untrusted.SigData,
	}, nil
}

// Hash returns the QuorumCertificate's identifier
func (qc *QuorumCertificate) Hash() Identifier {
	if qc == nil {
		return ZeroID
	}
	return MakeID(qc)
}

// Equals returns true if and only if receiver QuorumCertificate is equal to the `other`. Nil values are supported.
func (qc *QuorumCertificate) Equals(other *QuorumCertificate) bool {
	// Shortcut if `qc` and `other` point to the same object; covers case where both are nil.
	if qc == other {
		return true
	}
	if qc == nil || other == nil { // only one is nil, the other not (otherwise we would have returned above)
		return false
	}
	// both are not nil, so we can compare the fields
	return (qc.View == other.View) &&
		(qc.BlockID == other.BlockID) &&
		bytes.Equal(qc.SignerIndices, other.SignerIndices) &&
		bytes.Equal(qc.SigData, other.SigData)
}

// QuorumCertificateWithSignerIDs is a QuorumCertificate, where the signing nodes are
// identified via their `flow.Identifier`s instead of indices. Working with IDs as opposed to
// indices is less efficient, but simpler, because we don't require a canonical node order.
// It is used for bootstrapping new Epochs, because the FlowEpoch smart contract has no
// notion of node ordering.
type QuorumCertificateWithSignerIDs struct {
	View      uint64
	BlockID   Identifier
	SignerIDs []Identifier
	SigData   []byte
}
