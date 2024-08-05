package flow

// SetEpochExtensionViewCount is a service event emitted by the FlowServiceAccount to update `EpochExtensionViewCount`
// protocol parameter in the protocol state key-value store.
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

type EjectIdentity struct {
	NodeID Identifier
}

// EqualTo returns true if the two events are equivalent.
func (e *EjectIdentity) EqualTo(other *EjectIdentity) bool {
	return e.NodeID == other.NodeID
}

// ServiceEvent returns the event as a generic ServiceEvent type.
func (e *EjectIdentity) ServiceEvent() ServiceEvent {
	return ServiceEvent{
		Type:  ServiceEventEjectIdentity,
		Event: e,
	}
}
