package timeoutcollector

import (
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// TimeoutCertificateBuilder is a helper structure that stores intermediate state
// for building TC. It stores highest QC view contributed by each timeout and highest QC
// among all timeouts. Builder doesn't perform any verification of input data, resulting timeout certificate
// must be validated.
// This module is concurrently safe.
type TimeoutCertificateBuilder struct {
	lock        sync.Mutex
	view        uint64
	highQCViews []uint64
	highestQC   *flow.QuorumCertificate
}

func NewTimeoutCertificateBuilder(view uint64) *TimeoutCertificateBuilder {
	return &TimeoutCertificateBuilder{
		view:        view,
		highestQC:   nil,
		highQCViews: nil,
	}
}

// Add adds information about timeout into local cache
// CAUTION: doesn't perform any validation including deduplication.
func (b *TimeoutCertificateBuilder) Add(timeout *model.TimeoutObject) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.highQCViews = append(b.highQCViews, timeout.HighestQC.View)

	// don't update highestQC if timeout has lower QC
	if b.highestQC != nil && b.highestQC.View >= timeout.HighestQC.View {
		return
	}

	b.highestQC = timeout.HighestQC
}

// Build creates TimeoutCertificate using signers set, aggregated signature together and internally cached data
// CAUTION: there is no guarantee that resulting TC is valid, it must be validated before further processing.
func (b *TimeoutCertificateBuilder) Build(signers []flow.Identifier, aggregatedSig crypto.Signature) *flow.TimeoutCertificate {
	b.lock.Lock()
	defer b.lock.Unlock()
	return &flow.TimeoutCertificate{
		View:          b.view,
		TOHighQCViews: b.highQCViews,
		TOHighestQC:   b.highestQC,
		SignerIDs:     signers,
		SigData:       aggregatedSig,
	}
}
