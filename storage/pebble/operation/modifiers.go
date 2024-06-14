package operation

import "github.com/cockroachdb/pebble"

func OnlyWrite(write func(pebble.Writer) error) func(PebbleReaderWriter) error {
	return func(rw PebbleReaderWriter) error {
		return write(rw)
	}
}
