package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/stdlib"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	. "github.com/onsi/gomega"
	"testing"
)

func TestCheckNever(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheckWithExtra(
		`
            fun test(): Int {
                return panic("XXX")
            }
        `,
		stdlib.StandardLibraryFunctions{
			stdlib.PanicFunction,
		}.ToValueDeclarations(),
		nil,
		nil,
	)

	Expect(err).
		To(Not(HaveOccurred()))
}
