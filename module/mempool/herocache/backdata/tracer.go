package herocache

type CacheOpt[K comparable, V any] func(*Cache[K, V])

// Tracer is a generic interface that is used to report specific events that happen during
// lifetime of Cache and are potentially interesting for external consumer.
type Tracer[K comparable, V any] interface {
	// EntityEjectionDueToEmergency reports ejected entity whenever a bucket is found full and all of its keys are valid, i.e.,
	// each key belongs to an existing (key, entity) pair.
	// Hence, adding a new key to that bucket will replace the oldest valid key inside that bucket.
	// This ejection happens with very low, but still cryptographically non-negligible probability.
	EntityEjectionDueToEmergency(ejectedEntity V)
	// EntityEjectionDueToFullCapacity reports ejected entity whenever adding a new (key, entity) to the cache results in ejection of another (key', entity') pair.
	// This normally happens -- and is expected -- when the cache is full.
	EntityEjectionDueToFullCapacity(ejectedEntity V)
}

// WithTracer injects tracer into the cache
func WithTracer[K comparable, V any](t Tracer[K, V]) CacheOpt[K, V] {
	return func(c *Cache[K, V]) {
		c.tracer = t
	}
}
