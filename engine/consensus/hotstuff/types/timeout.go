package types

type TimeoutMode int

const (
	WaitingForProposalTimeout TimeoutMode = iota
	WaitingForVotesTimeout
)

type Timeout struct {
	Mode TimeoutMode
	View uint64
}
