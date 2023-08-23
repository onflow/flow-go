package storage

import (
	"github.com/onflow/flow-go/model/flow"
	"io"
)

type Registers interface {
	RegistersReader
	RegistersWriter
	io.Closer
}

type RegistersReader interface {
	Get(ID flow.RegisterID, height uint64) flow.RegisterEntry
}

type RegistersWriter interface {
	Store(entry flow.RegisterEntry, height uint64) error
}
