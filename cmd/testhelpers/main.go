// "testhelpers" binary exposes an http server to perform side-effects requested by integration tests
// The motivation for a separate library is to prevent this logic to ever end up running in production

package main

import (
	"fmt"
	// "log"
	"net/http"
	// execConfig "github.com/dapperlabs/bamboo-node/internal/execute/config"
	// execData "github.com/dapperlabs/bamboo-node/internal/execute/data"
	// secConfig "github.com/dapperlabs/bamboo-node/internal/security/config"
	// secData "github.com/dapperlabs/bamboo-node/internal/security/data"
)

func main() {

	// TODO: these 2 lines can't work together because they use the same set of env variables. Namespacing env variables is one possible solution.

	// execDAL := execData.New(execConfig.New())
	// secDAL := secData.New(secConfig.New())

	http.HandleFunc("/reset-db", func(w http.ResponseWriter, r *http.Request) {
		/*
			err := execDAL.MigrateDown()
			if err != nil {
				renderError(w, err, 500)
				return
			}

			err = execDAL.MigrateUp()
			if err != nil {
				renderError(w, err, 500)
				return
			}
		*/

		w.WriteHeader(200)
	})

	http.ListenAndServe(fmt.Sprintf(":%v", 3009), nil) // TODO: create config  just for this binary
}

/*
func renderError(w http.ResponseWriter, err error, status int) {
	log.Println(fmt.Sprintf("%v: %v", status, err))
	w.WriteHeader(status)
}
*/
