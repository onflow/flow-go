package runtime

import (
	"encoding/gob"
	"encoding/hex"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
)

type Location ast.Location

type StringLocation string

func (l StringLocation) ID() ast.LocationID {
	return ast.LocationID(l)
}

type AddressLocation ast.AddressLocation

func init() {
	gob.Register(AddressLocation{})
}

func (l AddressLocation) ID() ast.LocationID {
	return ast.LocationID(l.String())
}

func (l AddressLocation) String() string {
	return hex.EncodeToString(l)
}

type TransactionLocation []byte

func (l TransactionLocation) ID() ast.LocationID {
	return ast.LocationID(l.String())
}

func (l TransactionLocation) String() string {
	return hex.EncodeToString(l)
}

type ScriptLocation []byte

func (l ScriptLocation) ID() ast.LocationID {
	return ast.LocationID(l.String())
}

func (l ScriptLocation) String() string {
	return hex.EncodeToString(l)
}

type FileLocation string

func (l FileLocation) ID() ast.LocationID {
	return ast.LocationID(l.String())
}

func (l FileLocation) String() string {
	return string(l)
}

type REPLLocation struct{}

func (l REPLLocation) ID() ast.LocationID {
	return ast.LocationID(l.String())
}

func (l REPLLocation) String() string {
	return "REPL"
}
