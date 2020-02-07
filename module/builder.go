// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package module

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// BuildFunc is a build function provided by the consensus algorithm that
// creates the block proposal as makes sense within the consensus algorithm.
type BuildFunc func(payloadHash flow.Identifier) (*flow.Header, error)

// Builder represents an abstracted block construction module that can be used
// in more than one consensus algorithm. The resulting block is consistent
// within itself and can be wrapped with additional consensus information such
// as QCs.
type Builder interface {
	// BuildOn generates a new payload that is valid with respect to the parent
	// being built upon, builds and stores the header, then returns the stored
	// header.
	//
	// The caller is responsible for defining how to build and sign the header
	// by specifying the build function.
	BuildOn(parentID flow.Identifier, build BuildFunc) (*flow.Header, error)
}
