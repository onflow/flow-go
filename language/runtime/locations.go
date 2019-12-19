package runtime

import (
	"encoding/gob"
	"encoding/hex"
	"fmt"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
)

type (
	Location        = ast.Location
	LocationID      = ast.LocationID
	StringLocation  = ast.StringLocation
	AddressLocation = ast.AddressLocation
)

const (
	AddressPrefix            = ast.AddressPrefix
	TransactionPrefix string = "T"
	ScriptPrefix      string = "S"
)

// TransactionLocation

type TransactionLocation []byte

func (l TransactionLocation) ID() ast.LocationID {
	return LocationID(fmt.Sprintf(
		"%s.%s",
		TransactionPrefix,
		l.String(),
	))
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
	return LocationID(fmt.Sprintf(
		"%s.%s",
		ScriptPrefix,
		l.String(),
	))
}

func (l ScriptLocation) String() string {
	return hex.EncodeToString(l)
}

// FileLocation

type FileLocation string

func (l FileLocation) ID() ast.LocationID {
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
