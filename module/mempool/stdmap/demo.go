package stdmap

import (
	"fmt"
	"testing"

	"github.com/onflow/flow-go/model/flow"
)

func Test(t *testing.T) {
	cb := func(key flow.Identifier, entity flow.Entity) {
		fmt.Println("element ejected")
	}

	NewIncorporatedResultSeals(1000, WithEjectionCallback(cb)) // fine
	NewIncorporatedResults(WithEjectionCallback(cb))           // bug
}
