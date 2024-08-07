package flow

// SetEpochExtensionViewCount is a service event emitted by the FlowServiceAccount for updating
// the `EpochExtensionViewCount` parameter in the protocol state's key-value store.
// NOTE: A SetEpochExtensionViewCount event `E` is accepted while processing block `B`
// which seals `E` if and only if E.Value > 2*FinalizationSafetyThreshold.
type SetEpochExtensionViewCount struct {
	Value uint64
}

// EqualTo returns true if the two events are equivalent.
func (s *SetEpochExtensionViewCount) EqualTo(other *SetEpochExtensionViewCount) bool {
	return s.Value == other.Value
}

// ServiceEvent returns the event as a generic ServiceEvent type.
func (s *SetEpochExtensionViewCount) ServiceEvent() ServiceEvent {
	return ServiceEvent{
		Type:  ServiceEventSetEpochExtensionViewCount,
		Event: s,
	}
}
