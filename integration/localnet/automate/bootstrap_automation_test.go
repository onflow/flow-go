package automate

import (
	"fmt"
	"testing"
)

func TestGeneratedDataAccess(t *testing.T) {
	fmt.Printf("Starting tests")
	loadNodeJsonData()
}

func TestString(t *testing.T) {
	name := fmt.Sprint("access", 1)
	fmt.Println("Starting tests")
	fmt.Println(name)
}
