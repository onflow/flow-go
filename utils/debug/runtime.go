package debug

import (
	"runtime/metrics"
)

// GetHeapAllocsBytes returns the value of /gc/heap/allocs:bytes.
func GetHeapAllocsBytes() uint64 {
	// https://pkg.go.dev/runtime/metrics#hdr-Supported_metrics
	sample := []metrics.Sample{{Name: "/gc/heap/allocs:bytes"}}
	metrics.Read(sample)

	if sample[0].Value.Kind() == metrics.KindUint64 {
		return sample[0].Value.Uint64()
	}
	return 0
}
