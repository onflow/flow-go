package mocklocal

import (
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

// MockLocal represents a mock of Local
// We needed to develop a separate mock for Local as we could not mock
// a method with return values with gomock
type MockLocal struct {
	sk crypto.PrivateKey
	t  mock.TestingT
	id flow.Identifier
}

func NewMockLocal(sk crypto.PrivateKey, id flow.Identifier, t mock.TestingT) *MockLocal {
	return &MockLocal{
		sk: sk,
		t:  t,
		id: id,
	}
}

func (m *MockLocal) NodeID() flow.Identifier {
	return m.id
}

func (m *MockLocal) Address() string {
	require.Fail(m.t, "should not call MockLocal Address")
	return ""
}

func (m *MockLocal) Sign(msg []byte, hasher hash.Hasher) (crypto.Signature, error) {
	return m.sk.Sign(msg, hasher)
}

func (m *MockLocal) MockNodeID(id flow.Identifier) {
	m.id = id
}

func (m *MockLocal) NotMeFilter() flow.IdentityFilter {
	return filter.Not(filter.HasNodeID(m.id))
}

func (m *MockLocal) SignFunc(data []byte, hasher hash.Hasher, f func(crypto.PrivateKey, []byte, hash.Hasher) (crypto.Signature,
	error)) (crypto.Signature, error) {
	return f(m.sk, data, hasher)
}
