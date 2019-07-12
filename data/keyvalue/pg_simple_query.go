package keyvalue

import (
	"github.com/go-pg/pg"
)

// simpleQuery implements the Query interface.
type simpleQuery struct {
	db    *pg.DB
	query string
	isGet bool
}

func (sq *simpleQuery) Execute(params ...string) (result string, err error) {
	if sq.isGet {
		var value string
		_, err := sq.db.QueryOne(&value, sq.query, params)
		return value, err
	}

	_, execErr := sq.db.Exec(sq.query, params)
	return "", execErr
}
