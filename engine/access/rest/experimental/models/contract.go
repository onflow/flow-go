package models

import accessmodel "github.com/onflow/flow-go/model/access"

// Build populates a [Contract] from a domain model.
func (c *Contract) Build(contract *accessmodel.Contract) {
	c.Identifier = contract.Identifier
	c.Body = contract.Body
}
