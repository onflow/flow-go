package migration

import (
	_ "embed"
)

//go:embed Migration.cdc
var contractCode string

const ContractName = "Migration"

func ContractCode() []byte {
	return []byte(contractCode)
}
