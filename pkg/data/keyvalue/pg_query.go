package keyvalue

import (
	"fmt"

	"github.com/go-pg/pg"
)

// pgQuery implements the Query interface.
type pgQuery struct {
	db         *pg.DB
	query      string
	isGet      bool
	paramCount int
}

func (pg *pgQuery) Execute(params ...string) (result string, err error) {
	if err := pg.checkParams(params); err != nil {
		return "", err
	}

	if pg.isGet {
		var value string
		_, err := pg.db.QueryOne(&value, pg.query, params)
		return value, err
	}

	_, execErr := pg.db.Exec(pg.query, params)
	return "", execErr
}

func (pg *pgQuery) checkParams(params []string) error {
	if len(params) != pg.paramCount {
		return fmt.Errorf("Expected to substituted %d params, but received %d", pg.paramCount, len(params))
	}
	return nil
}

func (pg *pgQuery) debug(params []string) (string, error) {
	if err := pg.checkParams(params); err != nil {
		return "", err
	}
	return pg.query, nil
}
