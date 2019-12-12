package runtime

import (
	"encoding/gob"
	"encoding/hex"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
)

type Location = ast.Location

type LocationID = ast.LocationID

type StringLocation = ast.StringLocation

type AddressLocation = ast.AddressLocation

// TransactionLocation

type TransactionLocation []byte

func (l TransactionLocation) ID() LocationID {
	return LocationID(l.String())
}

func (l TransactionLocation) String() string {
	return hex.EncodeToString(l)
}

func init() {
	gob.Register(TransactionLocation{})
}

// ScriptLocation

type ScriptLocation []byte

func (l ScriptLocation) ID() LocationID {
	return LocationID(l.String())
}

func (l ScriptLocation) String() string {
	return hex.EncodeToString(l)
}

// FileLocation

type FileLocation string

func (l FileLocation) ID() LocationID {
	return LocationID(l.String())
}

func (l FileLocation) String() string {
	return string(l)
}

// REPLLocation

type REPLLocation struct{}

func (l REPLLocation) ID() LocationID {
	return LocationID(l.String())
}

func (l REPLLocation) String() string {
	return "REPL"
}
