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
//		def func() {
//			// bad, because the definition of err might get overwritten
//			err = closeAndMergeError(closable, err)
//		}()
//
// Better to use as below:
// func XXX() (
//
//	errToReturn error,
//	) {
//		def func() {
//			// good, because the error to returned is only updated here, and guaranteed to be returned
//			errToReturn = closeAndMergeError(closable, errToReturn)
//		}()
func CloseAndMergeError(closable io.Closer, err error) error {
	var merr *multierror.Error
	if err != nil {
		merr = multierror.Append(merr, err)
	}

	closeError := closable.Close()
	if closeError != nil {
		merr = multierror.Append(merr, closeError)
	}

	return merr.ErrorOrNil()
}
