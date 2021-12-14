package validators

import (
	"fmt"
	"github.com/onflow/flow-go/engine/access/rest"
)

func Height(r *rest.Request) error {
	h := r.GetQueryParam("height")
	if h == "" {
		return fmt.Errorf("missing height")
	}
	// todo write more validators

	return nil
}

func Address(r *rest.Request) error {
	a := r.GetVar("address")
	if a == "" {
		return fmt.Errorf("missing address")
	}
	// todo write more validators

	return nil
}

// explicit group validator would allow you to make sure only one of possible group is provided in request
// func ExplicitGroup([]string{"ids"}, []string{"start_height", "end_height"}
