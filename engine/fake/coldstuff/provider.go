// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package coldstuff

// Provider provides the consensus algorithm with the current payload to use in
// the next block proposal.
type Provider interface {
	Payload() ([]byte, error)
}
