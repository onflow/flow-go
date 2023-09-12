package registers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_NewMVCCComparer_Split(t *testing.T) {
	t.Parallel()

	comparer := NewMVCCComparer()

	tests := []struct {
		name string
		arg  []byte
		want int
	}{
		{name: "nil", arg: nil, want: -HeightSuffixLen},
		{name: "empty", arg: []byte(""), want: -HeightSuffixLen},
		{name: "edge0", arg: []byte("1234567"), want: -1},
		{name: "edge1", arg: []byte("12345678"), want: 0},
		{name: "edge2", arg: []byte("123456789"), want: 1},
		{name: "split", arg: []byte("1234567890"), want: 2},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, comparer.Split(tt.arg))
		})
	}
}
