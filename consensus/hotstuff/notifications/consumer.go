package notifications

// TelemetryLogger collects all consensus notifications, except for protocol violations.
// Functionally, it corresponds to the `Consumer` minus the methods in
// `ProposalViolationConsumer`, `VoteAggregationViolationConsumer`, `TimeoutAggregationViolationConsumer`.
// Implementations must:
//   - be concurrency safe
//   - be non-blocking
//   - handle repetition of the same events (with some processing overhead).
//type TelemetryLogger interface {
//	hotstuff.ViewLifecycleConsumer
//	hotstuff.CommunicatorConsumer
//	hotstuff.FinalizationConsumer
//	hotstuff.VoteCollectorConsumer
//	hotstuff.TimeoutCollectorConsumer
//}
