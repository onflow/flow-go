package alsp

import "github.com/onflow/flow-go/network"

const (
	// StaleMessage is a misbehavior that is reported when an engine receives a message that is deemed stale based on the
	// local view of the engine. The decision to consider a message stale is up to the engine.
	StaleMessage network.Misbehavior = "misbehavior-stale-message"

	// HeavyRequest is a misbehavior that is reported when an engine receives a request that takes an unreasonable amount
	// of resources by the engine to process, e.g., a request for a large number of blocks. The decision to consider a
	// request heavy is up to the engine.
	HeavyRequest network.Misbehavior = "misbehavior-heavy-request"

	// RedundantMessage is a misbehavior that is reported when an engine receives a message that is redundant, i.e., the
	// message is already known to the engine. The decision to consider a message redundant is up to the engine.
	RedundantMessage network.Misbehavior = "misbehavior-redundant-message"

	// UnsolicitedMessage is a misbehavior that is reported when an engine receives a message that is not solicited by the
	// engine. The decision to consider a message unsolicited is up to the engine.
	UnsolicitedMessage network.Misbehavior = "misbehavior-unsolicited-message"
)

func AllMisbehaviorTypes() []network.Misbehavior {
	return []network.Misbehavior{
		StaleMessage,
		HeavyRequest,
		RedundantMessage,
		UnsolicitedMessage,
	}
}
