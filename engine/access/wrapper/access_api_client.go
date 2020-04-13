package wrapper

import "github.com/dapperlabs/flow/protobuf/go/flow/access"

// AccessAPIClient allows for generation of a mock (via mockery) for the real AccessAPIClient imported as a dependency
// from the flow repo
type AccessAPIClient interface {
	access.AccessAPIClient
}
