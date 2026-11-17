package main

import (
	"fmt"
	"os"

	gcurl "github.com/glynnbird/gocurl/internal/app"
)

func main() {

	// create new gocurl struct
	goCurl, err := gcurl.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// run it
	err = goCurl.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
