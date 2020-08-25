package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type StorageCollector struct {
	storageOperationModified *prometheus.CounterVec
}

const (
	modifierLabel = "modifier"

	skipDuplicates  = "skip_duplicates"
	retryOnConflict = "retry_on_conflict"
)

var (
	onlyOnce  sync.Once
	collector *StorageCollector
)

func GetStorageCollector() *StorageCollector {
	onlyOnce.Do(
		func() {
			collector = &StorageCollector{
				storageOperationModified: promauto.NewCounterVec(prometheus.CounterOpts{
					Name:      "operation_modifier",
					Namespace: "storage",
					Subsystem: "badger",
					Help:      "report number of times a storage operation was modified using a modifier",
				}, []string{modifierLabel}),
			}
		},
	)

	return collector
}

func (sc *StorageCollector) SkipDuplicate() {
	sc.storageOperationModified.With(prometheus.Labels{modifierLabel: skipDuplicates}).Inc()
}

func (sc *StorageCollector) RetryOnConflict() {
	sc.storageOperationModified.With(prometheus.Labels{modifierLabel: retryOnConflict}).Inc()
}
