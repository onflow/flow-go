// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"github.com/dapperlabs/flow-go/module"
)

// OptionFunc is a function that can be provided to the backend on creation in
// order to set a certain custom option.
type OptionFunc func(*Backend)

// WithLimit can be provided to the backend on creation in order to set a custom
// maximum limit of entities in the memory pool.
func WithLimit(limit uint) OptionFunc {
	return func(be *Backend) {
		be.limit = limit
	}
}

// WithEject can be provided to the backend on creation in order to set a custom
// eject function to pick the entity to be evicted upon overflow, as well as
// hooking into it for additional cleanup work.
func WithEject(eject EjectFunc) OptionFunc {
	return func(be *Backend) {
		be.eject = eject
	}
}

// WithMetrics can be provided to the backend on creation in order to set a custom metric collector.
// metricResourceLabel is the label that the metric is identified with. As a convention, it represents
// the name of mempool that is implemented on top of this backend instance.
func WithMetrics(collector module.MempoolMetrics, metricResourceLabel string) OptionFunc {
	return func(be *Backend) {
		be.collector = collector
		be.metricResourceLabel = metricResourceLabel
	}
}
