package tests

//
//func TestCheckResource(t *testing.T) {
//	RegisterTestingT(t)
//
//	_, err := ParseAndCheck(`
//      resource R {}
//
//      fun test() {
//          let r: <-R <- R()
//          f(r)
//          f(r)
//      }
//
//      fun f(_ r: <-R) {}
//	`)
//
//	// TODO: add support for resources
//
//	errs := ExpectCheckerErrors(err, 1)
//
//	Expect(errs[0]).
//		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
//}
