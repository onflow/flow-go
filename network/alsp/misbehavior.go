package alsp

// Misbehavior is a type of misbehavior that can be reported by a node.
// The misbehavior is used to penalize the misbehaving node at the protocol level.
type Misbehavior string

func (m Misbehavior) String() string {
	return string(m)
}

const (
	// StaleMessage is a misbehavior that is reported when an engine receives a message that is deemed stale based on the
	// local view of the engine.
	StaleMessage Misbehavior = "misbehavior-stale-message"

	// HeavyRequest is a misbehavior that is reported when an engine receives a request that takes an unreasonable amount
	// of resources by the engine to process, e.g., a request for a large number of blocks. The decision to consider a
	// request heavy is up to the engine.
	HeavyRequest Misbehavior = "misbehavior-heavy-request"

	// RedundantMessage is a misbehavior that is reported when an engine receives a message that is redundant, i.e., the
	// message is already known to the engine. The decision to consider a message redundant is up to the engine.
	RedundantMessage Misbehavior = "misbehavior-redundant-message"

	// UnsolicitedMessage is a misbehavior that is reported when an engine receives a message that is not solicited by the
	// engine. The decision to consider a message unsolicited is up to the engine.
	UnsolicitedMessage Misbehavior = "misbehavior-unsolicited-message"
)
