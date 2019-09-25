// The testhelpers binary exposes an http server to perform side-effects requested by integration tests.
// The motivation for a separate library is to prevent this logic from ever running in production.

package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/psiemens/sconfig"

	"github.com/dapperlabs/flow-go/pkg/data/keyvalue"
)

// DBConfig holds the configuration for a Postgres database connection.
type DBConfig struct {
	PostgresAddr     string `default:"127.0.0.1:5432"`
	PostgresUser     string `default:"postgres"`
	PostgresPassword string `default:""`
	PostgresDatabase string `default:"postgres"`
}

func main() {
	collectDB := connectToDB("BAM_COLLECT")

	http.HandleFunc("/reset-db", func(w http.ResponseWriter, r *http.Request) {
		err := collectDB.MigrateDown()
		if err != nil {
			renderError(w, err, 500)
			return
		}

		err = collectDB.MigrateUp()
		if err != nil {
			renderError(w, err, 500)
			return
		}

		w.WriteHeader(200)
	})

	http.ListenAndServe(fmt.Sprintf(":%v", 3009), nil) // TODO: create config  just for this binary
}

// connectToDB opens a connection to a database for a specific service.
func connectToDB(service string) keyvalue.DBConnector {
	conf := parseConfigForService(service)
	return keyvalue.NewpostgresDB(
		conf.PostgresAddr,
		conf.PostgresUser,
		conf.PostgresPassword,
		conf.PostgresDatabase,
	)
}

// parseConfigForService parses database configuration from the environment for a specific
// service.
//
// For example, parseConfigForService("BAM_COLLECT") will read the following environment variables:
// - BAM_COLLECT_POSTGRESADDR
// - BAM_COLLECT_POSTGRESUSER
// - BAM_COLLECT_POSTGRESPASSWORD
// - BAM_COLLECT_POSTGRESDATABASE
func parseConfigForService(service string) DBConfig {
	var conf DBConfig

	err := sconfig.New(&conf).
		FromEnvironment(service).
		Parse()

	if err != nil {
		panic(err.Error())
	}

	return conf
}

func renderError(w http.ResponseWriter, err error, status int) {
	log.Println(fmt.Sprintf("%v: %v", status, err))
	w.WriteHeader(status)
}
