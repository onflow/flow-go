package mocksiface_test

import (
	"github.com/onflow/flow-go-sdk/access"
)

// This is a proxy for the real access.Client for mockery to use.
type Client interface {
	access.Client
}
