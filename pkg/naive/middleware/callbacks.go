// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package middleware

import (
	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/pkg/naive/overlay"
)

// defaultAddress is the default callback when the middleware layer needs an
// address to connect to. It errors explicitly to force the overlay layer to
// implement and register a proper address callback.
func defaultAddress() (string, error) {
	return "", errors.New("no address callback registered")
}

// defaultHandshake is the default callback when the middleware layer needs to
// perform a handshake on a new connection. It errors explicitly to force the
// overlay layer to implement and register a proper handshake callback.
func defaultHandshake(conn overlay.Connection) (string, error) {
	return "", errors.New("no address callback registered")
}

// defaultCleanup is the default callback to notify the overlay layer that a
// connection was dropped and related resources can be cleaned up. It errors
// explicitly to force the overlay layer to implement proper cleanup procedures.
func defaultCleanup(node string) error {
	return errors.New("no cleanup callback registered")
}

// defaultReceive is the default callback to notify the overlay layer about new
// messages. It errors explicitly to force the overlay layer to implement and
// register message handling.
func defaultReceive(node string, msg interface{}) error {
	return errors.New("no receive callback registered")
}
