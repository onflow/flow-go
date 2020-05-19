// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

type Cleaner interface {
	RunGC()
}
