package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/model/flow"
)

func TestDomainTags(t *testing.T) {
	assert.Equal(
		t,
		len(flow.TransactionDomainTag),
		len(flow.UserDomainTag),
		"tag lengths must be equal",
	)
}
