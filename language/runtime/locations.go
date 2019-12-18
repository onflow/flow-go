package runtime

import (
	"encoding/gob"
	"encoding/hex"
	"fmt"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
)

type Location = ast.Location

type LocationID = ast.LocationID

type StringLocation = ast.StringLocation

type AddressLocation = ast.AddressLocation

// TransactionLocation

type TransactionLocation []byte

func (l TransactionLocation) ID() ast.LocationID {
	return ast.LocationID(fmt.Sprintf("T.%s", l))
}

func (l TransactionLocation) String() string {
	return hex.EncodeToString(l)
}

func init() {
	gob.Register(TransactionLocation{})
}

// ScriptLocation

type ScriptLocation []byte

func (l ScriptLocation) ID() ast.LocationID {
	return ast.LocationID(fmt.Sprintf("S.%s", l))
}

func (l ScriptLocation) String() string {
	return hex.EncodeToString(l)
}

// FileLocation

type FileLocation string

func (l FileLocation) ID() ast.LocationID {
	return ast.LocationID(fmt.Sprintf("F.%s", l))
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
