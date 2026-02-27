package models

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"

	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	accessmodel "github.com/onflow/flow-go/model/access"
)

// Build populates the REST model from the domain model.
func (m *ContractDeployment) Build(d *accessmodel.ContractDeployment, link LinkGenerator) {
	m.ContractId = d.ContractID
	m.Address = d.Address.Hex()
	m.BlockHeight = strconv.FormatUint(d.BlockHeight, 10)
	m.TransactionId = d.TransactionID.String()
	m.TxIndex = strconv.FormatUint(uint64(d.TxIndex), 10)
	m.EventIndex = strconv.FormatUint(uint64(d.EventIndex), 10)
	m.CodeHash = hex.EncodeToString(d.CodeHash)
	if len(d.Code) > 0 {
		m.Code = base64.StdEncoding.EncodeToString(d.Code)
	}

	m.Expandable = new(ContractDeploymentExpandable)
	if d.Transaction != nil {
		m.Transaction = new(commonmodels.Transaction)
		m.Transaction.Build(d.Transaction, nil, link)
	} else {
		transactionLink, err := link.TransactionLink(d.TransactionID)
		if err != nil {
			panic(fmt.Errorf("failed to generate transaction link: %w", err))
		}
		m.Expandable.Transaction = transactionLink
	}
}
