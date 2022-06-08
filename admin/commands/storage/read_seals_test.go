package storage

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/model/flow"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestReadSealsByID(t *testing.T) {
	t.Parallel()

	state := new(protocolmock.State)
	seals := new(storagemock.Seals)
	index := new(storagemock.Index)

	seal := unittest.Seal.Fixture()
	seals.On("ByID", mock.AnythingOfType("flow.Identifier")).Return(
		func(sealID flow.Identifier) *flow.Seal {
			if sealID == seal.ID() {
				return seal
			}
			return nil
		},
		func(sealID flow.Identifier) error {
			if sealID == seal.ID() {
				return nil
			}
			return fmt.Errorf("seal %#v not found", sealID)
		},
	)

	command := NewReadSealsCommand(state, seals, index)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &admin.CommandRequest{
		Data: map[string]interface{}{
			"seal": seal.ID().String(),
		},
	}
	require.NoError(t, command.Validator(req))
	result, err := command.Handler(ctx, req)
	require.NoError(t, err)

	resultMap, err := commands.ConvertToMap(seal)
	require.NoError(t, err)

	assert.DeepEqual(t, result, resultMap)
}
