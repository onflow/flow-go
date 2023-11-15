package notifications

import "github.com/onflow/flow-go/consensus/hotstuff"

// TelemetryConsumer collects all consensus notifications emitted by `hotstuff.Consumer`, except for
// protocol violations. Functionally, it corresponds to the `hotstuff.Consumer` *minus* the methods in
// `ProposalViolationConsumer`, `VoteAggregationViolationConsumer`, `TimeoutAggregationViolationConsumer`.
// Implementations must:
//   - be concurrency safe
//   - be non-blocking
//   - handle repetition of the same events (with some processing overhead).
type TelemetryConsumer interface {
	hotstuff.ViewLifecycleConsumer
	hotstuff.CommunicatorConsumer
	hotstuff.FinalizationConsumer
	hotstuff.VoteCollectorConsumer
	hotstuff.TimeoutCollectorConsumer
}

// SlashingViolationsConsumer collects all consensus notifications emitted by `hotstuff.Consumer`
// that are related to protocol violations.
// Implementations must:
//   - be concurrency safe
//   - be non-blocking
//   - handle repetition of the same events (with some processing overhead).
type SlashingViolationsConsumer interface {
	hotstuff.ProposalViolationConsumer
	hotstuff.VoteAggregationViolationConsumer
	hotstuff.TimeoutAggregationViolationConsumer
}
