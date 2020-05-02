// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
)

type Cleaner struct {
	log   zerolog.Logger
	db    *badger.DB
	ratio float64
}

func NewCleaner(log zerolog.Logger, db *badger.DB) *Cleaner {
	c := &Cleaner{
		log:   log.With().Str("component", "cleaner").Logger(),
		db:    db,
		ratio: 0.5,
	}
	return c
}

func (c *Cleaner) RunGC() {
	go func() {
		err := c.db.RunValueLogGC(c.ratio)
		if err != nil {
			c.log.Error().Err(err).Msg("could not run KV DB GC")
		}
	}()
}
