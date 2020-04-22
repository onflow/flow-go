package integration

import (
	"errors"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/pacemaker/timeout"
	"github.com/dapperlabs/flow-go/model/flow"
)

var errStopCondition = errors.New("stop condition reached")

type Option func(*Config)

type Config struct {
	Root              *flow.Header
	Participants      flow.IdentityList
	LocalID           flow.Identifier
	Timeouts          timeout.Config
	IncomingVotes     VoteFilter
	OutgoingVotes     VoteFilter
	IncomingProposals ProposalFilter
	OutgoingProposals ProposalFilter
	StopCondition     Condition
}

func WithRoot(root *flow.Header) Option {
	return func(cfg *Config) {
		cfg.Root = root
	}
}

func WithParticipants(participants flow.IdentityList) Option {
	return func(cfg *Config) {
		cfg.Participants = participants
	}
}

func WithLocalID(localID flow.Identifier) Option {
	return func(cfg *Config) {
		cfg.LocalID = localID
	}
}

func WithTimeouts(timeouts timeout.Config) Option {
	return func(cfg *Config) {
		cfg.Timeouts = timeouts
	}
}

func WithIncomingVotes(Filter VoteFilter) Option {
	return func(cfg *Config) {
		cfg.IncomingVotes = Filter
	}
}

func WithOutgoingVotes(Filter VoteFilter) Option {
	return func(cfg *Config) {
		cfg.OutgoingVotes = Filter
	}
}

func WithIncomingProposals(Filter ProposalFilter) Option {
	return func(cfg *Config) {
		cfg.IncomingProposals = Filter
	}
}

func WithOutgoingProposals(Filter ProposalFilter) Option {
	return func(cfg *Config) {
		cfg.OutgoingProposals = Filter
	}
}

func WithStopCondition(stop Condition) Option {
	return func(cfg *Config) {
		cfg.StopCondition = stop
	}
}
