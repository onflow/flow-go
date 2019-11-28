package runtime

import (
	"encoding/hex"
	"fmt"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
)

type Location ast.Location

type StringLocation string

func (l StringLocation) ID() ast.LocationID {
	return ast.LocationID(l)
}

type AddressLocation ast.AddressLocation

func (l AddressLocation) ID() ast.LocationID {
	return ast.LocationID(fmt.Sprintf("A.%s", l))
}

func (l AddressLocation) String() string {
	return hex.EncodeToString(l)
}

type TransactionLocation []byte

func (l TransactionLocation) ID() ast.LocationID {
	return ast.LocationID(fmt.Sprintf("T.%s", l))
}

func (l TransactionLocation) String() string {
	return hex.EncodeToString(l)
}

type ScriptLocation []byte

func (l ScriptLocation) ID() ast.LocationID {
	return ast.LocationID(fmt.Sprintf("S.%s", l))
}

func (l ScriptLocation) String() string {
	return hex.EncodeToString(l)
}

type FileLocation string

func (l FileLocation) ID() ast.LocationID {
	return ast.LocationID(fmt.Sprintf("F.%s", l))
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
