package fvm

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/model/flow"
)

func Test_simpleAddressState_Bytes(t *testing.T) {
	s := SimpleAddressState{513}
	assert.Equal(t, []byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x2, 0x1}, s.Bytes())
}

func Test_simpleAddressState_NextAddress(t *testing.T) {
	s := SimpleAddressState{}

	addr, err := s.NextAddress()
	assert.NoError(t, err)
	assert.Equal(t, flow.Address{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}, addr)

	addr = s.CurrentAddress()
	assert.Equal(t, flow.Address{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}, addr)

	addr, err = s.NextAddress()
	assert.NoError(t, err)
	assert.Equal(t, flow.Address{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02}, addr)
}

func Test_bytesToSimpleAddressState(t *testing.T) {
	type args struct {
		b []byte
	}
	tests := []struct {
		name string
		args args
		want SimpleAddressState
	}{
		{
			name: "works for nil",
			args: args{b: nil},
			want: SimpleAddressState{lastAddress: 0},
		},
		{
			name: "works for empty",
			args: args{b: []byte{}},
			want: SimpleAddressState{lastAddress: 0},
		},
		{
			name: "works for 0",
			args: args{b: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
			want: SimpleAddressState{lastAddress: 0},
		},
		{
			name: "works for 1",
			args: args{b: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}},
			want: SimpleAddressState{lastAddress: 1},
		},
		{
			name: "works for 511",
			args: args{b: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0xff}},
			want: SimpleAddressState{lastAddress: 511},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BytesToSimpleAddressState(tt.args.b); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("bytesToSimpleAddressState() = %v, want %v", got, tt.want)
			}
		})
	}
}
