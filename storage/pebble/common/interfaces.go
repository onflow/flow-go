package common

import "github.com/cockroachdb/pebble"

type PebbleReaderWriter interface {
	pebble.Reader
	pebble.Writer
}
