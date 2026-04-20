package debug

import (
	"encoding/hex"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

// testOwner returns the raw binary owner string for a given hex address string,
// matching the encoding used by AddressToRegisterOwner.
func testOwner(hexAddr string) string {
	return string(flow.HexToAddress(hexAddr).Bytes())
}

func TestFileRegisterCache_NewEmptyCache(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cache, err := NewFileRegisterCache(filepath.Join(dir, "cache.dat"))
	require.NoError(t, err)
	require.NotNil(t, cache)
}

func TestFileRegisterCache_SetGet(t *testing.T) {
	t.Parallel()

	owner := testOwner("0000000000000001")
	key := "someKey"
	value := []byte("someValue")

	dir := t.TempDir()
	cache, err := NewFileRegisterCache(filepath.Join(dir, "cache.dat"))
	require.NoError(t, err)

	_, found := cache.Get(owner, key)
	assert.False(t, found)

	cache.Set(owner, key, value)

	got, found := cache.Get(owner, key)
	require.True(t, found)
	assert.Equal(t, value, got)
}

func TestFileRegisterCache_SetDoesNotAliasValue(t *testing.T) {
	t.Parallel()

	owner := testOwner("0000000000000001")
	key := "k"
	value := []byte("original")

	dir := t.TempDir()
	cache, err := NewFileRegisterCache(filepath.Join(dir, "cache.dat"))
	require.NoError(t, err)

	cache.Set(owner, key, value)
	value[0] = 'X' // mutate original slice

	got, found := cache.Get(owner, key)
	require.True(t, found)
	assert.Equal(t, byte('o'), got[0], "Set should store a copy of value")
}

func TestFileRegisterCache_PersistAndReload(t *testing.T) {
	t.Parallel()

	owner := testOwner("0000000000000001")
	key := "someKey"
	value := []byte("someValue")

	dir := t.TempDir()
	filePath := filepath.Join(dir, "cache.dat")

	cache, err := NewFileRegisterCache(filePath)
	require.NoError(t, err)
	cache.Set(owner, key, value)
	require.NoError(t, cache.Persist())

	loaded, err := NewFileRegisterCache(filePath)
	require.NoError(t, err)

	got, found := loaded.Get(owner, key)
	require.True(t, found)
	assert.Equal(t, value, got)
}

func TestFileRegisterCache_ParsedEntryKey(t *testing.T) {
	t.Parallel()

	owner := testOwner("0000000000000001")
	key := "someKey"
	value := []byte("someValue")

	dir := t.TempDir()
	filePath := filepath.Join(dir, "cache.dat")

	cache, err := NewFileRegisterCache(filePath)
	require.NoError(t, err)
	cache.Set(owner, key, value)
	require.NoError(t, cache.Persist())

	loaded, err := NewFileRegisterCache(filePath)
	require.NoError(t, err)

	entry, found := loaded.data[newRegisterKey(owner, key)]
	require.True(t, found)

	// Owner is stored as raw binary (AddressToRegisterOwner encoding).
	assert.Equal(t, owner, entry.Key.Owner)

	// Key is stored as hex-encoded bytes; decode it to recover the original key.
	decoded, err := hex.DecodeString(entry.Key.Key)
	require.NoError(t, err)
	assert.Equal(t, key, string(decoded))
}

func TestFileRegisterCache_MultipleEntries(t *testing.T) {
	t.Parallel()

	owner1 := testOwner("0000000000000001")
	owner2 := testOwner("0000000000000002")

	dir := t.TempDir()
	filePath := filepath.Join(dir, "cache.dat")

	cache, err := NewFileRegisterCache(filePath)
	require.NoError(t, err)
	cache.Set(owner1, "k1", []byte("v1"))
	cache.Set(owner1, "k2", []byte("v2"))
	cache.Set(owner2, "k1", []byte("v3"))
	require.NoError(t, cache.Persist())

	loaded, err := NewFileRegisterCache(filePath)
	require.NoError(t, err)

	v, found := loaded.Get(owner1, "k1")
	require.True(t, found)
	assert.Equal(t, []byte("v1"), v)

	v, found = loaded.Get(owner1, "k2")
	require.True(t, found)
	assert.Equal(t, []byte("v2"), v)

	v, found = loaded.Get(owner2, "k1")
	require.True(t, found)
	assert.Equal(t, []byte("v3"), v)

	_, found = loaded.Get(owner2, "k2")
	assert.False(t, found)
}
