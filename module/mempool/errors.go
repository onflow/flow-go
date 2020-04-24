// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package mempool

import (
	"errors"
)

var (
	ErrNotFound      = errors.New("entity not found in memory pool")
	ErrAlreadyExists = errors.New("entity already exists in memory pool")
)
