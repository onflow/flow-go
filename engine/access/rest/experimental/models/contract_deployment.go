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
func (m *ContractDeployment) Build(d *accessmodel.ContractDeployment, link LinkGenerator) error {
	m.ContractId = d.ContractID
	m.Address = d.Address.Hex()
	m.BlockHeight = strconv.FormatUint(d.BlockHeight, 10)
	m.TransactionId = d.TransactionID.String()
	m.TxIndex = strconv.FormatUint(uint64(d.TransactionIndex), 10)
	m.EventIndex = strconv.FormatUint(uint64(d.EventIndex), 10)
	m.CodeHash = hex.EncodeToString(d.CodeHash)
	if len(d.Code) > 0 {
		m.Code = base64.StdEncoding.EncodeToString(d.Code)
	}

	m.IsPlaceholder = d.IsPlaceholder

	m.Expandable = new(ContractDeploymentExpandable)
	if d.Transaction != nil {
		m.Transaction = new(commonmodels.Transaction)
		m.Transaction.Build(d.Transaction, nil, link)
	} else if !d.IsPlaceholder {
		transactionLink, err := link.TransactionLink(d.TransactionID)
		if err != nil {
			return fmt.Errorf("failed to generate transaction link: %w", err)
		}
		m.Expandable.Transaction = transactionLink
	}

	if d.Result != nil {
		m.Result = new(commonmodels.TransactionResult)
		m.Result.Build(d.Result, d.TransactionID, link)
	} else if !d.IsPlaceholder {
		resultLink, err := link.TransactionResultLink(d.TransactionID)
		if err != nil {
			return fmt.Errorf("failed to generate result link: %w", err)
		}
		m.Expandable.Result = resultLink
	}

	return nil
}
