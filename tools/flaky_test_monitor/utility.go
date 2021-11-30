package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
)

func assertErrNil(err error, panicMessage string) {
	if err != nil {
		panic(panicMessage + ": " + err.Error())
	}
}

func convertTo2DecimalPlaces2(numerator float32, denominator int) float32 {
	return convertTo2DecimalPlacesInternal(numerator, float32(denominator))
}

func convertTo2DecimalPlaces(numerator, denominator int) float32 {
	return convertTo2DecimalPlacesInternal(float32(numerator), float32(denominator))
}

func convertTo2DecimalPlacesInternal(numerator, denominator float32) float32 {
	ratioString := fmt.Sprintf("%.2f", numerator/denominator)
	ratioFloat, err := strconv.ParseFloat(ratioString, 32)
	assertErrNil(err, "failure parsing string to float")
	return float32(ratioFloat)
}

func folderExists(path string) bool {
	_, err := os.Stat(path)

	// folder exists if there is no error
	if err == nil {
		return true
	}

	// folder doesn't exist if error is of specific type
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}

	// should never get to here
	panic("error checking if folder exists:" + err.Error())
}
