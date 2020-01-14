// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package middleware

import (
	"io"
	"strings"

	"github.com/pkg/errors"
)

// isClosedErr detects network errors that mean the connection have been closed,
// which might well be intended.
func isClosedErr(err error) bool {
	if err == nil {
		return false
	}
	err = errors.Cause(err)
	if err == io.EOF {
		return true
	}
	msg := err.Error()
	if strings.Contains(msg, "use of closed network connection") {
		return true
	}
	if strings.Contains(msg, "connection reset by peer") {
		return true
	}
	return false
}
