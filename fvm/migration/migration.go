package migration

import (
	_ "embed"
	"fmt"
	"regexp"

	"github.com/onflow/flow-go/model/flow"
)

//go:embed Migration.cdc
var contractCode string

const ContractName = "Migration"

var accountV2MigrationImportPattern = regexp.MustCompile(`(?m)^import "AccountV2Migration"`)

func ContractCode(accountV2MigrationAddress flow.Address) []byte {
	evmContract := accountV2MigrationImportPattern.ReplaceAllString(
		contractCode,
		fmt.Sprintf("import AccountV2Migration from %s", accountV2MigrationAddress.HexWithPrefix()),
	)
	return []byte(evmContract)
}
