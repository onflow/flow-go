package tracker

import (
	"unsafe"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// NewestQCTracker is a helper structure which keeps track of the newest QC(by view)
// in concurrency safe way.
type NewestQCTracker struct {
	newestQC *atomic.UnsafePointer
}

func NewNewestQCTracker() *NewestQCTracker {
	tracker := &NewestQCTracker{
		newestQC: atomic.NewUnsafePointer(unsafe.Pointer(nil)),
	}
	return tracker
}

// Track updates local state of NewestQC if the provided instance is newer(by view)
// Concurrently safe
func (t *NewestQCTracker) Track(qc *flow.QuorumCertificate) bool {
	// to record the newest value that we have ever seen we need to use loop
	// with CAS atomic operation to make sure that we always write the latest value
	// in case of shared access to updated value.
	for {
		// take a snapshot
		newestQC := t.NewestQC()
		// verify that our update makes sense
		if newestQC != nil && newestQC.View >= qc.View {
			return false
		}
		// attempt to install new value, repeat in case of shared update.
		if t.newestQC.CompareAndSwap(unsafe.Pointer(newestQC), unsafe.Pointer(qc)) {
			return true
		}
	}
}

// NewestQC returns the newest QC(by view) tracked.
// Concurrently safe.
func (t *NewestQCTracker) NewestQC() *flow.QuorumCertificate {
	return (*flow.QuorumCertificate)(t.newestQC.Load())
}

// NewestTCTracker is a helper structure which keeps track of the newest TC(by view)
// in concurrency safe way.
type NewestTCTracker struct {
	newestTC *atomic.UnsafePointer
}

func NewNewestTCTracker() *NewestTCTracker {
	tracker := &NewestTCTracker{
		newestTC: atomic.NewUnsafePointer(unsafe.Pointer(nil)),
	}
	return tracker
}

// Track updates local state of NewestTC if the provided instance is newer(by view)
// Concurrently safe.
func (t *NewestTCTracker) Track(tc *flow.TimeoutCertificate) bool {
	// to record the newest value that we have ever seen we need to use loop
	// with CAS atomic operation to make sure that we always write the latest value
	// in case of shared access to updated value.
	for {
		// take a snapshot
		newestTC := t.NewestTC()
		// verify that our update makes sense
		if newestTC != nil && newestTC.View >= tc.View {
			return false
		}
		// attempt to install new value, repeat in case of shared update.
		if t.newestTC.CompareAndSwap(unsafe.Pointer(newestTC), unsafe.Pointer(tc)) {
			return true
		}
	}
}

// NewestTC returns the newest TC(by view) tracked.
// Concurrently safe.
func (t *NewestTCTracker) NewestTC() *flow.TimeoutCertificate {
	return (*flow.TimeoutCertificate)(t.newestTC.Load())
}

// NewestBlockTracker is a helper structure which keeps track of the newest block (by view)
// in concurrency safe way.
type NewestBlockTracker struct {
	newestBlock *atomic.UnsafePointer
}

func NewNewestBlockTracker() *NewestBlockTracker {
	tracker := &NewestBlockTracker{
		newestBlock: atomic.NewUnsafePointer(unsafe.Pointer(nil)),
	}
	return tracker
}

// Track updates local state of newestBlock if the provided instance is newer (by view)
// Concurrently safe.
func (t *NewestBlockTracker) Track(block *model.Block) bool {
	// to record the newest value that we have ever seen we need to use loop
	// with CAS atomic operation to make sure that we always write the latest value
	// in case of shared access to updated value.
	for {
		// take a snapshot
		newestBlock := t.NewestBlock()
		// verify that our update makes sense
		if newestBlock != nil && newestBlock.View >= block.View {
			return false
		}
		// attempt to install new value, repeat in case of shared update.
		if t.newestBlock.CompareAndSwap(unsafe.Pointer(newestBlock), unsafe.Pointer(block)) {
			return true
		}
	}
}

// NewestBlock returns the newest block (by view) tracked.
// Concurrently safe.
func (t *NewestBlockTracker) NewestBlock() *model.Block {
	return (*model.Block)(t.newestBlock.Load())
}

// NewestPartialTcTracker tracks the newest partial TC (by view) in a concurrency safe way.
type NewestPartialTcTracker struct {
	newestPartialTc *atomic.UnsafePointer
}

func NewNewestPartialTcTracker() *NewestPartialTcTracker {
	tracker := &NewestPartialTcTracker{
		newestPartialTc: atomic.NewUnsafePointer(unsafe.Pointer(nil)),
	}
	return tracker
}

// Track updates local state of newestPartialTc if the provided instance is newer (by view)
// Concurrently safe.
func (t *NewestPartialTcTracker) Track(partialTc *hotstuff.PartialTcCreated) bool {
	// To record the newest value that we have ever seen, we need to use loop
	// with CAS atomic operation to make sure that we always write the latest value
	// in case of shared access to updated value.
	for {
		// take a snapshot
		newestPartialTc := t.NewestPartialTc()
		// verify that our partial TC is from a newer view
		if newestPartialTc != nil && newestPartialTc.View >= partialTc.View {
			return false
		}
		// attempt to install new value, repeat in case of shared update.
		if t.newestPartialTc.CompareAndSwap(unsafe.Pointer(newestPartialTc), unsafe.Pointer(partialTc)) {
			return true
		}
	}
}

// NewestPartialTc returns the newest partial TC (by view) tracked.
// Concurrently safe.
func (t *NewestPartialTcTracker) NewestPartialTc() *hotstuff.PartialTcCreated {
	return (*hotstuff.PartialTcCreated)(t.newestPartialTc.Load())
}
