package storage

import (
	"testing"
)

func TestMakeSingletonLockManager_PanicsWhenCalledTwice(t *testing.T) {
	// First call should succeed.
	_ = MakeSingletonLockManager()

	// Second call should panic.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on second MakeSingletonLockManager call, but got none")
		}
	}()
	_ = MakeSingletonLockManager()
}
