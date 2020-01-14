// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package mempool

import (
	"errors"
)

var (
	ErrEntityNotFound      = errors.New("entity not found in memory pool")
	ErrEntityAlreadyExists = errors.New("entity already exists in memory pool")
)
