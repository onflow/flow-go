// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"fmt"

	"github.com/pkg/errors"
)

// Role represents a role in the flow system.
type Role uint8

// Enumeration of the available flow node roles.
const (
	RoleCollection   Role = 1
	RoleConsensus    Role = 2
	RoleExecution    Role = 3
	RoleVerification Role = 4
	RoleAccess       Role = 5
)

// String returns a string version of role.
func (r Role) String() string {
	switch r {
	case RoleCollection:
		return "collection"
	case RoleConsensus:
		return "consensus"
	case RoleExecution:
		return "execution"
	case RoleVerification:
		return "verification"
	case RoleAccess:
		return "access"
	default:
		panic(fmt.Sprintf("invalid role (%d)", r))
	}
}

// ParseRole will parse a role from string.
func ParseRole(role string) (Role, error) {
	switch role {
	case "collection":
		return RoleCollection, nil
	case "consensus":
		return RoleConsensus, nil
	case "execution":
		return RoleExecution, nil
	case "verification":
		return RoleVerification, nil
	case "access":
		return RoleAccess, nil
	default:
		return 0, errors.Errorf("invalid role string (%s)", role)
	}
}

func (r Role) MarshalText() ([]byte, error) {
	return []byte(r.String()), nil
}

func (r *Role) UnmarshalText(text []byte) error {
	var err error
	*r, err = ParseRole(string(text))
	return err
}
