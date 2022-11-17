package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBTreeView(t *testing.T) {
	v1 := NewBTreeView()
	_ = v1.Set("a", "b", []byte("c"))
	actual1, _ := v1.Get("a", "b")
	require.Equal(t, []byte("c"), actual1)

	v2 := v1.NewChild()
	_ = v2.Set("a", "b", []byte("d"))
	actual2, _ := v2.Get("a", "b")
	require.Equal(t, []byte("d"), actual2)

	v3 := v2.NewChild()
	_ = v3.Delete("a", "b")
	actual3, _ := v3.Get("a", "b")
	require.Nil(t, actual3)

	v3.DropDelta()
	actual3_p, _ := v3.Get("a", "b")
	require.Equal(t, []byte("d"), actual3_p)

	_ = v1.MergeView(v2)
	actual1_p, _ := v1.Get("a", "b")
	require.Equal(t, []byte("d"), actual1_p)
}
