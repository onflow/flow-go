package merr

import (
	"io"

	"github.com/hashicorp/go-multierror"
)

// CloseAndMergeError close the closable and merge the closeErr with the given err into a multierror
// Note: when using this function in a defer function, don't use as below:
// func XXX() (
//
//	err error,
//	) {
//		defer func() {
//			// bad, because the definition of err might get overwritten by another deferred function
//			err = closeAndMergeError(closable, err)
//		}()
//
// Better to use as below:
// func XXX() (
//
//	errToReturn error,
//	) {
//		defer func() {
//			// good, because the error to returned is only updated here, and guaranteed to be returned
//			errToReturn = closeAndMergeError(closable, errToReturn)
//		}()
func CloseAndMergeError(closable io.Closer, err error) error {
	return multierror.Append(err, closable.Close()).ErrorOrNil()
}
