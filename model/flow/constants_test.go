package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/model/flow"
)

func TestDomainTags(t *testing.T) {
	assert.Len(t, flow.TransactionDomainTag, flow.DomainTagLength)
	assert.Len(t, flow.UserDomainTag, flow.DomainTagLength)
}
