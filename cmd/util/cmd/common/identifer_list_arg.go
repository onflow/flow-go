package common

import (
	"fmt"
	"strings"

	"github.com/onflow/flow-go/model/flow"
)

// IdentifierListValue defines a custom command line spf13 pflag to accept a list of flow identifiers as a comma separated list of
// hex string
// e.g. -nodeid=b4a4dbdcd443d0ed1938bd3adec17aea9be65d8f0d57871bdd71ae8bc63d7fd1,fb386a6ad47f5c69022a335c0d78a94a77eee04bb5fe781bf3a6c092d72390ca
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
