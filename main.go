package main

import (
	"fmt"
	MV "github.com/maroda/monteverdi/server"
)

func main() {
	setting := MV.FillEnvVar("HOME")
	fmt.Printf("monteverdi, %s\n", setting)
}
