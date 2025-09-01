package main

import (
	"fmt"
	MV "github.com/maroda/monteverdi/server"
)

func main() {
	User := MV.FillEnvVar("USER")
	fmt.Printf("Welcome to Monteverdi, %s\n", User)

	A01 := MV.NewAccent(1, "main", "output")

	// View some struct data
	fmt.Printf("Accent full timestamp: %s\n", A01.Timestamp)

	// Use an interface method
	fmt.Printf("Interface timestamp: %s\n", A01.TimestampString())

	// Print out the accent
	fmt.Printf("Accent mark '%d' entered for '%s' to be displayed on '%s'\n", A01.Intensity, A01.SourceID, A01.DestLayer)
}
