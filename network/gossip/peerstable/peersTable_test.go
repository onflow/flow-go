package peerstable

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestPeersTableAdd(t *testing.T) {
	pt, err := NewPeersTable()

	require.Nil(t, err, "failed to create peersTable")

	IP := "127.0.0.1:1999"
	ID := unittest.IdentifierFixture()

	var ok bool

	_, ok = pt.fromIDToIP[ID]
	assert.False(t, ok, "ID to IP table should not contain unadded values")

	_, ok = pt.fromIPToID[IP]
	assert.False(t, ok, "IP to ID table should not contain unadded values")

	pt.Add(ID, IP)

	nIP, ok := pt.fromIDToIP[ID]
	assert.True(t, ok, "ID to IP table should contain added values")
	assert.Equal(t, nIP, IP, "added IP should be linked with the correlated ID")

	nID, ok := pt.fromIPToID[IP]
	assert.True(t, ok, "IP to ID table should contain added values")
	assert.Equal(t, nID, ID, "added ID should be linked with the correlated IP")
}

func TestPeersTableGetIP(t *testing.T) {
	pt, err := NewPeersTable()

	require.Nil(t, err, "failed to create peersTable")

	id1 := unittest.IdentifierFixture()
	id2 := unittest.IdentifierFixture()

	tt := []struct {
		IP string
		ID model.Identifier
	}{
		{IP: "127.0.0.1:1999", ID: id1},
		{IP: "127.0.0.1:1998", ID: id2},
	}

	for _, tc := range tt {
		pt.fromIDToIP[tc.ID] = tc.IP
		nIP, err := pt.GetIP(tc.ID)
		require.Nil(t, err, "failed to get IP from ID")

		assert.Equal(t, nIP, tc.IP, "added IP should be the same as retrieved IP")
	}
}

func TestPeersTableGetIPS(t *testing.T) {
	pt, err := NewPeersTable()

	require.Nil(t, err, "failed to create peersTable")

	id1 := unittest.IdentifierFixture()
	id2 := unittest.IdentifierFixture()
	id3 := unittest.IdentifierFixture()

	pt.fromIDToIP = map[model.Identifier]string{
		id1: "127.0.0.1:1999",
		id2: "127.0.0.1:1998",
		id3: "127.0.0.1:1997",
	}

	IPs := []string{
		"127.0.0.1:1999",
		"127.0.0.1:1998",
		"127.0.0.1:1997",
	}

	nIPs, err := pt.GetIPs(id1, id2, id3)
	require.Nil(t, err, "failed to get IP from ID")

	assert.Equal(t, IPs, nIPs, "added IPs should be the same as retrieved IPs")
}

func TestPeersTableGetID(t *testing.T) {
	pt, err := NewPeersTable()

	require.Nil(t, err, "failed to create peersTable")

	id1 := unittest.IdentifierFixture()
	id2 := unittest.IdentifierFixture()

	tt := []struct {
		ID model.Identifier
		IP string
	}{
		{ID: id1, IP: "127.0.0.1:1999"},
		{ID: id2, IP: "127.0.0.1:1998"},
	}

	for _, tc := range tt {
		pt.fromIPToID[tc.IP] = tc.ID
		nID, err := pt.GetID(tc.IP)
		require.Nil(t, err, "failed to get ID from IP")

		assert.Equal(t, nID, tc.ID, "added ID should be the same as retrieved ID")
	}
}

func TestPeersTableGetIDS(t *testing.T) {
	pt, err := NewPeersTable()

	require.Nil(t, err, "failed to create peersTable")

	id1 := unittest.IdentifierFixture()
	id2 := unittest.IdentifierFixture()
	id3 := unittest.IdentifierFixture()

	pt.fromIPToID = map[string]model.Identifier{
		"127.0.0.1:1999": id1,
		"127.0.0.1:1998": id2,
		"127.0.0.1:1997": id3,
	}

	IDs := []model.Identifier{id1, id2, id3}

	nIDs, err := pt.GetIDs("127.0.0.1:1999", "127.0.0.1:1998", "127.0.0.1:1997")
	require.Nil(t, err, "failed to get ID from IP")

	assert.Equal(t, IDs, nIDs, "added IDs should be the same as retrieved IDs")
}
