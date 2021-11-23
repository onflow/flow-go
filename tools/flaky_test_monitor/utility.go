package main

func assertErrNil(err error, panicMessage string) {
	if err != nil {
		panic(panicMessage + err.Error())
	}
}
