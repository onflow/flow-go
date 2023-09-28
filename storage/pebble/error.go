package pebble

import "github.com/pkg/errors"

var ErrNotBootstrapped = errors.New("pebble database not bootstrapped")
