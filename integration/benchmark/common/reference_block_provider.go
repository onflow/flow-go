package common

import flowsdk "github.com/onflow/flow-go-sdk"

type ReferenceBlockProvider interface {
	// ReferenceBlockID returns the reference block ID of a recent block.
	ReferenceBlockID() flowsdk.Identifier
}
