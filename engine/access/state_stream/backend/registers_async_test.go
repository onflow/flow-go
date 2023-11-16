package backend

import (
	"testing"

	"github.com/onflow/flow-go/model/flow"
	state_synchronization "github.com/onflow/flow-go/module/state_synchronization/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RegistersAsyncSuite struct {
	BackendExecutionDataSuite
}

func TestRegistersAsyncSuite(t *testing.T) {
	suite.Run(t, new(RegistersAsyncSuite))
}

func (r *RegistersAsyncSuite) SetupTest() {
	r.BackendExecutionDataSuite.SetupTest()
}

func (r *RegistersAsyncSuite) TestInitDataAvailable() {
	// test data available on init
	registerID := unittest.RegisterIDFixture()
	registerValue1 := []byte("response1")
	registerValue2 := []byte("response2")
	require.True(r.T(), r.registersAsync.initialized.Load(), false)
	firstHeight := r.backend.rootBlockHeight
	latestHeight := r.backend.rootBlockHeight + 1
	r.backend.setHighestHeight(latestHeight)

	registers := storagemock.NewRegisterIndex(r.T())
	registers.On("FirstHeight").Return(firstHeight)
	registers.On("LatestHeight").Return(latestHeight)
	registers.On("Get", registerID, firstHeight).Return(registerValue1)
	registers.On("Get", registerID, latestHeight).Return(registerValue2)

	indexReporter := state_synchronization.NewIndexReporter(r.T())
	indexReporter.On("LowestIndexedHeight").Return(firstHeight)
	indexReporter.On("HighestIndexedHeight").Return(latestHeight)

	// registersDB bootstrapped, correct values returned
	r.registersAsync.InitDataAvailable(indexReporter, registers)
	require.True(r.T(), r.registersAsync.initialized.Load(), true)
	val1, err := r.registersAsync.RegisterValues([]flow.RegisterID{registerID}, firstHeight)
	require.NoError(r.T(), err)
	require.Equal(r.T(), val1[0], registerValue1)

	val2, err := r.registersAsync.RegisterValues([]flow.RegisterID{registerID}, latestHeight)
	require.NoError(r.T(), err)
	require.Equal(r.T(), val2[0], registerValue2)

	// out of bounds height, correct error returned
	_, err = r.registersAsync.RegisterValues([]flow.RegisterID{registerID}, latestHeight+1)
	require.ErrorIs(r.T(), err, storage.ErrHeightNotIndexed)

	// no register value available, correct error returned
	invalidRegisterID := flow.RegisterID{
		Owner: "ha",
		Key:   "ha",
	}
	_, err = r.registersAsync.RegisterValues([]flow.RegisterID{invalidRegisterID}, latestHeight)
	require.ErrorIs(r.T(), err, storage.ErrNotFound)
}

func (r *RegistersAsyncSuite) TestRegisterValuesDataUnAvailable() {
	// registerDB not bootstrapped, correct error returned
	registerID := unittest.RegisterIDFixture()
	require.True(r.T(), r.registersAsync.initialized.Load(), false)
	_, err := r.registersAsync.RegisterValues([]flow.RegisterID{registerID}, r.backend.rootBlockHeight)
	require.ErrorIs(r.T(), err, storage.ErrHeightNotIndexed)
}
