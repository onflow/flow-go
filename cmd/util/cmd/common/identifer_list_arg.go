package common

import (
	"fmt"
	"strings"

	"github.com/onflow/flow-go/model/flow"
)

// IdentifierListValue defines a custom command line flag to accept a list of flow identifiers as a comma separated list of
// hex string
type IdentifierListValue flow.IdentifierList

func (f *IdentifierListValue) Type() string {
	return "Flow Identifier list"
}

func (f *IdentifierListValue) String() string {
	return fmt.Sprintf("%s", flow.IdentifierList(*f).Strings())
}

func (f *IdentifierListValue) Set(value string) error {
	ss := strings.Split(value, ",")
	for _, s := range ss {
		id, err := flow.HexStringToIdentifier(s)
		if err != nil {
			return fmt.Errorf("failed to covert %s to a flow identifier: %w", value, err)
		}
		*f = append(*f, id)
	}
	return nil
}
