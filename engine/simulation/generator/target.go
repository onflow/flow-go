// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package generator

// Target represents an engine we want to submit this type to.
type Target interface {
	Submit(interface{})
}
