package persisters

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/store/inmemory/unsynchronized"
	"github.com/onflow/flow-go/utils/unittest"
)

// RegistersPersisterSuite tests the RegistersPersister separately since it uses a different database
type RegistersPersisterSuite struct {
	suite.Suite
	persister         *RegistersPersister
	inMemoryRegisters *unsynchronized.Registers
	registers         *storagemock.RegisterIndex
	header            *flow.Header
}

func TestRegistersPersisterSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(RegistersPersisterSuite))
}

func (r *RegistersPersisterSuite) SetupTest() {
	block := unittest.BlockFixture()
	r.header = block.ToHeader()

	r.inMemoryRegisters = unsynchronized.NewRegisters(r.header.Height)
	r.registers = storagemock.NewRegisterIndex(r.T())

	r.persister = NewRegistersPersister(
		r.inMemoryRegisters,
		r.registers,
		r.header.Height,
	)
}

func (r *RegistersPersisterSuite) TestRegistersPersister_PersistWithEmptyData() {
	// Registers must be stored for every height, even if empty
	storedRegisters := make([]flow.RegisterEntry, 0)
	r.registers.On("Store", mock.Anything, r.header.Height).Run(func(args mock.Arguments) {
		sr, ok := args.Get(0).(flow.RegisterEntries)
		r.Require().True(ok)
		storedRegisters = sr
	}).Return(nil).Once()

	err := r.persister.Persist()
	r.Require().NoError(err)

	// Verify empty registers were stored
	r.Assert().Empty(storedRegisters)
	r.registers.AssertExpectations(r.T())
}

func (r *RegistersPersisterSuite) TestRegistersPersister_PersistWithData() {
	// Populate register data
	regEntries := make(flow.RegisterEntries, 3)
	for i := 0; i < 3; i++ {
		regEntries[i] = unittest.RegisterEntryFixture()
	}
	err := r.inMemoryRegisters.Store(regEntries, r.header.Height)
	r.Require().NoError(err)

	// Setup mock to capture stored data
	storedRegisters := make([]flow.RegisterEntry, 0)
	r.registers.On("Store", mock.Anything, r.header.Height).Run(func(args mock.Arguments) {
		sr, ok := args.Get(0).(flow.RegisterEntries)
		r.Require().True(ok)
		storedRegisters = sr
	}).Return(nil).Once()

	err = r.persister.Persist()
	r.Require().NoError(err)

	// Verify the correct data was stored
	expectedRegisters, err := r.inMemoryRegisters.Data(r.header.Height)
	r.Require().NoError(err)
	r.Assert().ElementsMatch(expectedRegisters, storedRegisters)
	r.registers.AssertExpectations(r.T())
}

func (r *RegistersPersisterSuite) TestRegistersPersister_ErrorHandling() {
	tests := []struct {
		name          string
		setupMocks    func()
		expectedError string
	}{
		{
			name: "RegistersStoreError",
			setupMocks: func() {
				r.registers.On("Store", mock.Anything, r.header.Height).Return(assert.AnError).Once()
			},
			expectedError: "could not persist registers",
		},
		{
			name: "RegistersDataError",
			setupMocks: func() {
				// Create a persisters with wrong height to trigger data error
				wrongPersister := NewRegistersPersister(
					r.inMemoryRegisters,
					r.registers,
					r.header.Height+1, // Wrong height
				)
				r.persister = wrongPersister
			},
			expectedError: "could not get data from registers",
		},
	}

	for _, test := range tests {
		r.Run(test.name, func() {
			test.setupMocks()

			err := r.persister.Persist()
			r.Require().Error(err)

			r.Assert().Contains(err.Error(), test.expectedError)
		})
	}
}
